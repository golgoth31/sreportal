# Your Network Topology Is a Black Box — Until It Isn't

> This is part 5 of 7 of the *SRE Portal in Practice* series.
> [Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)

---

## A Question Nobody Can Answer Cleanly

A new engineer joins the payments team. Smart, motivated, asking the right questions. On day two they ask: "Which services call the payments API?"

You know roughly. You were there when the architecture was designed, you remember a Slack thread from six months ago, you vaguely recall a NetworkPolicy someone wrote for the checkout service. But explaining the actual, enforced, currently-deployed network flows requires reading through dozens of NetworkPolicy YAMLs across multiple namespaces — and that's before you factor in the Cilium policies governing external egress.

So you say "roughly the checkout service and the subscription worker, maybe the reporting job" — and you're already unsure. The engineer writes it down, and that note becomes the new source of truth. Until it isn't.

This isn't a documentation problem. It's a visibility problem. NetworkPolicies are enforced live by the kernel and the CNI plugin, but there's no native Kubernetes API that says "here is the current graph of allowed flows." The enforcement exists. The graph doesn't.

SRE Portal's Network Flow Discovery feature builds that graph automatically, from the policies you've already written.

---

## The Concept

Network Flow Discovery is built around the `NetworkFlowDiscovery` CRD. You create one per Portal, point it at the namespaces you want to scan, and the controller does the rest: it reads every NetworkPolicy (and FQDNNetworkPolicy for Cilium-managed external endpoints), and translates ingress and egress rules into a graph of nodes and edges.

**Nodes** represent workloads. The controller classifies each one:
- `service` — a standard Kubernetes service
- `cron` — a CronJob
- `database` — a stateful workload (detected by labels or naming conventions)
- `messaging` — a message broker
- `external` — an external endpoint from a FQDNNetworkPolicy

**Edges** represent allowed traffic flows between nodes. Each edge carries a type:
- `internal` — both endpoints in the same namespace
- `cross-ns` — cross-namespace traffic
- `cron` — a CronJob initiating a connection
- `database` / `messaging` — flows to classified backend workloads
- `external` — egress to an FQDN endpoint outside the cluster

The resulting graph is stored in two cluster-scoped CRDs: `FlowNodeSet` and `FlowEdgeSet`. These are owned by the `NetworkFlowDiscovery` resource and hold the full topology per Portal.

**FlowObserver** goes one step further. A secondary observation loop probes each edge to check whether traffic actually flows — not just whether it's permitted. Each edge carries two fields after observation: `used` (traffic was observed) and `evaluated` (the observation was attempted). An edge with `evaluated: false` means that edge type can't be probed programmatically — external endpoints fall into this category. An edge with `used: false` but `evaluated: true` means the policy allows it, but nothing has sent traffic recently. That distinction matters when you're decommissioning a service.

For multi-cluster setups, `spec.isRemote: true` tells the controller to federate topology from a remote Portal via the Connect API rather than scraping local policies directly.

---

## In Practice

### Step 1 — Create a NetworkFlowDiscovery

```yaml
apiVersion: sreportal.io/v1alpha1
kind: NetworkFlowDiscovery
metadata:
  name: payments-flows
  namespace: sreportal-system
spec:
  portalRef: payments               # links to the Portal named "payments"

  # optional: restrict scanning to specific namespaces
  # empty (or omitted) means scan all namespaces
  namespaces:
    - payments
    - checkout
    - shared-infra

  # set to true to federate from a remote portal via Connect API
  # instead of reading local NetworkPolicy objects
  isRemote: false
```

The controller begins reconciling immediately. It scans the named namespaces (or all namespaces if the list is empty), reads every `NetworkPolicy` and `FQDNNetworkPolicy` object, and builds the graph.

### Step 2 — Inspect the status

After reconciliation, the `NetworkFlowDiscovery` status shows a summary:

```yaml
status:
  nodeCount: 14         # distinct workloads discovered
  edgeCount: 31         # allowed flows between them
  lastReconcileTime: "2026-05-14T09:12:04Z"
```

The actual graph data lives in the generated `FlowNodeSet` and `FlowEdgeSet` objects, both cluster-scoped and owned by this resource. You can inspect them directly:

```bash
kubectl get flownodeset payments-flows -o yaml
kubectl get flowedgeset payments-flows -o yaml
```

### What a FlowEdge looks like

Each edge in the `FlowEdgeSet` captures a single allowed flow, enriched by the FlowObserver:

```yaml
# excerpt from FlowEdgeSet status
edges:
  - source: checkout/checkout-api
    target: payments/payments-api
    type: cross-ns           # source and target are in different namespaces
    used: true               # FlowObserver confirmed traffic flowing
    evaluated: true          # observation was possible for this edge type

  - source: reporting/report-job
    target: payments/payments-api
    type: cron               # source is a CronJob
    used: false              # policy allows it, but no traffic observed recently
    evaluated: true

  - source: payments/payments-worker
    target: stripe.com
    type: external           # FQDNNetworkPolicy egress rule
    used: false
    evaluated: false         # external endpoints cannot be probed
```

The `used: false, evaluated: true` combination on the reporting job is immediately actionable: either the job stopped running and nobody noticed, or the policy was added speculatively and the dependency doesn't exist. Either way, you have something worth investigating. Previously, you'd have no idea.

### The web UI

Once the graph is built, it appears under the Portal in the SRE Portal web UI as an interactive network graph. Nodes are rendered with their classification (icons distinguish services, cron jobs, databases, messaging systems, and external endpoints). Edges are color-coded by type.

Click a node to filter the graph to only that node's connections — useful when your topology has 80 services and you only care about the blast radius of one. Filter by namespace to isolate cross-namespace traffic. Toggle the FlowObserver overlay to dim edges where `used: false` and focus on what's actually live.

The graph updates each reconciliation cycle. There's nothing to deploy, no agent to run alongside your services, no metrics pipeline to wire up. The controller reads your existing NetworkPolicies and the FlowObserver does the rest.

---

## Why It Matters

**Onboarding becomes a 10-minute conversation.** "Which services call the payments API?" is now a click, not a research project. The new engineer sees the graph, not a list of YAML files.

**Decommissioning without guesswork.** Before you delete a service, you can see which edges are `used: true` pointing at it. If the answer is zero, you can decommission with confidence. If the answer is three, you have a list of callers to notify.

**Policy drift surfaces itself.** Policies that allow flows nobody actually uses accumulate over time. The `used: false, evaluated: true` state makes them visible without writing a custom audit tool.

**Federation for multi-cluster platforms.** With `isRemote: true`, a central Portal can aggregate topology across all your clusters. The payments team's graph can include traffic flows from the ordering cluster and the analytics cluster without requiring access to each one directly.

**AI-agent friendly.** The FlowNodeSet and FlowEdgeSet CRDs are queryable via the SRE Portal MCP server. An AI coding assistant can answer "can the checkout service reach the payments database?" by calling the API, not by asking you.

---

## What's Next

You understand the topology: what flows, what's live, what's allowed but idle. The next question is what to tell your users when something in that topology breaks. Episode 6 covers Status Pages — how SRE Portal lets you define components, incidents, and maintenance windows so you can communicate outages to your users without standing up a separate status page service.

**Source code:** [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal)

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart).*
