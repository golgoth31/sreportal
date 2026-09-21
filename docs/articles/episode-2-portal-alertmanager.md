# Your Alerts Have No Context — And That Costs You On-Call Time

*This is part 2 of 7 of the* ***SRE Portal in Practice*** *series.*
*[Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)*

---

## 3am. PagerDuty fires.

The alert title reads: `HighErrorRate`. Severity: critical. That's it.

You stare at the screen. Which service? Which cluster? Which team owns this? The runbook link in the alert takes you to a Confluence page that was last edited in 2022. You start digging through Grafana, cross-referencing namespace names, guessing which Alertmanager instance fired this based on the routing tree you half-remember from last quarter's configuration session.

Ten minutes pass. You still haven't touched the actual problem.

This is the alert context gap: your monitoring stack knows something is wrong, but it has no structured way to answer *what service this belongs to* — let alone who to wake up next, where the dashboard is, or what changed recently. The alert fired in isolation, disconnected from the rest of your service knowledge.

SRE Portal Episode 1 covered how the `DNS` and `DNSRecord` CRDs give you a structured map of every service endpoint in your cluster. This episode introduces the `Portal` and `Alertmanager` CRDs — the layer that groups services into logical units and connects your Alertmanager instances to that map.

---

## The Concept

### Portal: a logical grouping for everything you care about

A `Portal` is a named grouping object. Think of it as a team's lens on the platform: *payment-team*, *infra*, *data-platform*. Everything else — DNS records, alert sources, release trackers, image inventories — attaches to a Portal via a `portalRef`.

This solves a real problem: your platform hosts dozens of services, but an on-call engineer needs to narrow the blast radius fast. A Portal gives them a pre-defined scope.

The Portal CRD also has a feature gate system. You can selectively enable or disable DNS discovery, the alerts page, the releases view, network policy visualization, and the status page — per portal. A team that only wants alerts has no noise from the rest.

One Portal can be marked `main: true` — that's the catch-all for DNS entries that don't match a more specific portal.

### Alertmanager: connecting your monitoring stack

The `Alertmanager` CRD is a pointer to an Alertmanager instance, linked to a Portal via `portalRef`. The URL field has two sub-fields on purpose:

- `local`: what the operator uses to poll the Alertmanager API (`/api/v2/alerts`). This can be a cluster-internal address.
- `remote`: an optional externally-reachable URL the web UI can surface as a direct link.

The operator reconciles `Alertmanager` resources on a schedule. Each cycle it hits `/api/v2/alerts`, parses the response, and writes the active alerts — fingerprint, labels, annotations, state, timestamps, receivers, and silence references — directly into `status.activeAlerts`. The cluster becomes the single source of truth for what's currently firing, scoped to a portal.

Multiple `Alertmanager` CRDs can point to the same Portal. If you run separate Alertmanager instances per environment (staging and production share a portal, each with its own Alertmanager), both sets of alerts land in the same portal view.

---

## In Practice

### Step 1: Create a Portal

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Portal
metadata:
  name: payment-team
  namespace: sreportal-system
spec:
  # Human-readable name shown in the web UI
  title: "Payment Team"

  # Optional: disable features this team doesn't use
  features:
    dns: true
    alerts: true
    releases: true
    networkPolicy: false     # not needed for this team
    statusPage: false        # not yet rolled out
    imageInventory: true
```

The `Portal` resource itself is lightweight. Its job is to be the anchor — every other CRD references it by name. Once this is created, the web UI registers a `/payment-team` portal view automatically.

### Step 2: Connect an Alertmanager

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Alertmanager
metadata:
  name: payment-alertmanager
  namespace: sreportal-system
spec:
  # Links this Alertmanager to the portal created above
  portalRef: payment-team

  url:
    # Internal URL used by the operator to poll /api/v2/alerts
    local: "http://alertmanager.monitoring.svc.cluster.local:9093"

    # Optional: externally reachable URL surfaced as a link in the UI
    remote: "https://alertmanager.internal.example.com"
```

That's the full configuration. After the operator reconciles, `status.activeAlerts` will populate with whatever is currently firing in that Alertmanager instance:

```yaml
status:
  lastReconcileTime: "2026-05-14T03:12:00Z"
  activeAlerts:
    - fingerprint: "a1b2c3d4e5f6"
      state: active
      startsAt: "2026-05-14T02:58:00Z"
      labels:
        alertname: HighErrorRate
        severity: critical
        namespace: payment
        service: payment-api
      annotations:
        summary: "Error rate above 5% for 10 minutes"
        runbook: "https://wiki.internal/runbooks/payment-high-error-rate"
      receivers:
        - pagerduty-payment
```

### Step 3: What you see in the web UI

The Alerts page in the web UI is scoped per portal. When you navigate to the *Payment Team* portal, you see only alerts sourced from Alertmanager instances that reference `portalRef: payment-team`. Each alert displays:

- Severity badge (derived from the `severity` label)
- Alert name and summary annotation
- Labels as a filterable tag cloud (namespace, service, environment, etc.)
- Time since firing
- Which receivers it's routed to
- A direct link to the remote Alertmanager URL when configured

If you have two Alertmanager CRDs pointing to the same portal — say, one for production and one for staging — their alerts are aggregated into a single view, each tagged with whichever labels distinguish them (typically `env: prod` vs `env: staging`).

---

## Why It Matters

The Portal + Alertmanager pattern changes what on-call looks like in practice.

**Scope narrows immediately.** When you wake up to an alert, you open the portal for the affected team and see exactly what's firing there — not every alert in the company's monitoring stack.

**Labels stay human.** The operator stores raw alert labels as-is from Alertmanager. Your existing label taxonomy (`team`, `service`, `namespace`, `severity`) is preserved and searchable in the UI without any remapping.

**No extra tooling to deploy.** If you already run Alertmanager (Prometheus stack, Grafana Mimir, or standalone), you point the CRD at it. The operator handles polling; you don't change your Alertmanager configuration.

**The cluster is the source of truth.** Active alerts live in `.status.activeAlerts`. You can `kubectl get alertmanager payment-alertmanager -o yaml` during an incident and see what's firing without touching any external dashboard. Other operators, scripts, or GitOps workflows can read this as structured data.

**Multi-team, multi-cluster ready.** Each Portal is independent. You can have 20 teams, each with their own Portal and one or more Alertmanager CRDs, all in the same cluster. The web UI presents each team their own scoped view.

---

## What's Next

Once you know *what's* firing and *which service* it's on, the next question that comes up in every incident is: *what changed?*

Episode 3 covers the `Release` CRD — how SRE Portal tracks deployments and surfaces recent release history directly in the portal view. Because the question "did something just get deployed?" deserves a faster answer than digging through your CI/CD system at 3am.

The full source is at [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal).

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart)*
