---
title: Network Flow Discovery
weight: 7
---

Network Flow Discovery automatically builds a graph of service-to-service, service-to-database, and service-to-external flows by reading Kubernetes NetworkPolicies and GKE FQDNNetworkPolicies.

## How it works

1. A **NetworkFlowDiscovery** Custom Resource is created, linked to a Portal via `spec.portalRef` (auto-created at startup for the main portal)
2. The controller periodically reconciles (every 60s) and:
   - Lists all `NetworkPolicy` resources (ingress rules → service-to-service flows)
   - Lists all `FQDNNetworkPolicy` resources (egress rules → service-to-database/external flows)
   - Builds a graph of nodes and edges
   - Stores nodes in a **FlowNodeSet** CR and edges in a **FlowEdgeSet** CR (both owned by the parent)
3. The **web UI** and **MCP server** read from FlowNodeSet/FlowEdgeSet — no cluster queries at request time

## CRDs

### NetworkFlowDiscovery

The parent resource that drives reconciliation.

```yaml
apiVersion: sreportal.io/v1alpha1
kind: NetworkFlowDiscovery
metadata:
  name: netflow-main
  namespace: sreportal-system
spec:
  portalRef: main
  # Optional: restrict to specific namespaces (default: all)
  namespaces: []
```

**Status fields:**

| Field | Description |
|-------|-------------|
| `status.nodeCount` | Number of discovered nodes |
| `status.edgeCount` | Number of discovered edges |
| `status.lastReconcileTime` | When the graph was last rebuilt |
| `status.conditions` | Ready condition indicating reconciliation success |

### FlowNodeSet

Stores all discovered nodes. Auto-created and owned by the parent NetworkFlowDiscovery.

- Named `<parent>-nodes` (e.g. `netflow-main-nodes`)
- `spec.discoveryRef` references the parent
- `status.nodes[]` contains the discovered nodes

Each node has the following fields:

| Field | Description | Example |
|-------|-------------|---------|
| `id` | Unique identifier | `service:namespace-a:service-1` |
| `label` | Human-readable name | `service-1` |
| `namespace` | Kubernetes namespace | `namespace-a` |
| `nodeType` | Classification (see below) | `service` |
| `group` | Logical group (namespace) | `namespace-a` |

### FlowEdgeSet

Stores all discovered edges. Auto-created and owned by the parent NetworkFlowDiscovery.

- Named `<parent>-edges` (e.g. `netflow-main-edges`)
- `spec.discoveryRef` references the parent
- `status.edges[]` contains the discovered edges

Each edge has the following fields:

| Field | Description | Example |
|-------|-------------|---------|
| `from` | Source node id | `service:namespace-a:service-1` |
| `to` | Target node id | `service:namespace-b:service-2` |
| `edgeType` | Flow type (see below) | `cross-ns` |

Deleting the parent `NetworkFlowDiscovery` automatically garbage-collects both children.

### Node types

Nodes are classified by port:

| Type | Rule |
|------|------|
| `service` | Pods referenced in NetworkPolicy ingress rules (`app.kubernetes.io/name` label) |
| `cron` | CronJobs referenced in NetworkPolicy ingress rules (`basename` label) |
| `database` | FQDNs on ports 5432, 1433, 3306 |
| `messaging` | FQDNs on ports 5672, 5671 |
| `external` | All other FQDNs in egress rules |

### Edge types

| Type | Meaning |
|------|---------|
| `internal` | Flow between services in the same namespace |
| `cross-ns` | Flow between services in different namespaces |
| `cron` | Flow from a cron job to a service |
| `database` | Flow from a service to a database |
| `messaging` | Flow from a service to a messaging broker |
| `external` | Flow from a service to an external endpoint |

## Web UI

The Network Policies page (`/:portalName/netpol`) provides three views:

### Flow Matrix

Lists every service with its outgoing and incoming flows. Features:
- Collapsible namespace and service sections
- Search filter and namespace dropdown
- **Copy as Markdown** for documentation/audit export

### Cross-Namespace

Aggregated matrix showing flow counts between namespaces. Useful for understanding inter-team dependencies.

### Impact Analysis

Select any resource (database, service, external endpoint) to see its blast radius:
- **Level 1**: direct dependents (services that call the resource)
- **Level 2**: services that call Level 1 services
- And so on, up to configurable depth

## MCP Server

Available at `/mcp/netpol` with three tools:

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_network_flows` | List all nodes and edges in the flow graph | `search` |
| `get_service_flows` | Get incoming/outgoing flows for a specific service | `service` (required) |
| `impact_analysis` | Blast radius analysis for a resource | `resource` (required), `max_depth` |

## Kubernetes API calls

The controller makes exactly **two LIST calls** per reconciliation cycle (every 60s):

1. `GET /apis/networking.k8s.io/v1/networkpolicies`
2. `GET /apis/networking.gke.io/v1alpha3/fqdnnetworkpolicies` (silently skipped if CRD is absent)

The web UI and MCP server read only from FlowNodeSet/FlowEdgeSet CRDs — zero cluster queries at request time.
