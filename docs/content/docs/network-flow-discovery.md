---
title: Network Flow Discovery
weight: 7
---

Network Flow Discovery automatically builds a graph of service-to-service, service-to-database, and service-to-external flows by reading Kubernetes NetworkPolicies and GKE FQDNNetworkPolicies.

## How it works

1. A **NetworkFlowDiscovery** Custom Resource is created, linked to a Portal via `spec.portalRef`
2. The controller periodically reconciles (every 60s) and:
   - Lists all `NetworkPolicy` resources (ingress rules → service-to-service flows)
   - Lists all `FQDNNetworkPolicy` resources (egress rules → service-to-database/external flows)
   - Builds a graph of nodes and edges
   - Stores the result in the CRD status
3. The **web UI** and **MCP server** read from the CRD status — no cluster queries at request time

## CRD: NetworkFlowDiscovery

```yaml
apiVersion: sreportal.io/v1alpha1
kind: NetworkFlowDiscovery
metadata:
  name: main
  namespace: sreportal-system
spec:
  portalRef: main
  # Optional: restrict to specific namespaces (default: all)
  namespaces: []
```

### Status fields

| Field | Description |
|-------|-------------|
| `status.nodes[]` | All discovered nodes (services, crons, databases, messaging, external endpoints) |
| `status.edges[]` | Directional flow relations between nodes |
| `status.lastReconcileTime` | When the graph was last rebuilt |
| `status.conditions` | Ready condition indicating reconciliation success |

### Node types

Nodes are classified by their source:

| Type | Source |
|------|--------|
| `service` | Pods referenced in NetworkPolicy ingress rules (`app.kubernetes.io/name` label) |
| `cron` | CronJobs referenced in NetworkPolicy ingress rules (`basename` label) |
| `database` | FQDNs on ports 5432 (PostgreSQL), 1433 (MSSQL), 3306 (MySQL) |
| `messaging` | FQDNs on ports 5672/5671 (AMQP) |
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

### Example: Claude Code

```bash
# In your Claude Code MCP settings, add:
{
  "mcpServers": {
    "sreportal-netpol": {
      "url": "http://localhost:8090/mcp/netpol"
    }
  }
}
```

Then ask: *"What services depend on the payments database?"* or *"Show me the blast radius if collect-api goes down."*

## Kubernetes API calls

The controller makes exactly **two LIST calls** per reconciliation cycle:

1. `GET /apis/networking.k8s.io/v1/networkpolicies` — standard K8s NetworkPolicies
2. `GET /apis/networking.gke.io/v1alpha3/fqdnnetworkpolicies` — GKE FQDNNetworkPolicies (silently skipped if CRD is absent)

The web UI and MCP server read only from the CRD status — zero cluster queries at request time.
