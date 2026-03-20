---
title: Observability
weight: 4
---

SRE Portal exposes custom Prometheus metrics on the controller-runtime `/metrics` endpoint alongside the built-in Go runtime and controller-runtime metrics. A pre-built Grafana dashboard is included in the repository.

## Metrics Endpoint

The metrics endpoint is configured via the `--metrics-bind-address` flag:

```bash
# Disabled by default
--metrics-bind-address=0

# HTTP
--metrics-bind-address=:8080

# HTTPS (auto-generated or cert-manager certificates)
--metrics-bind-address=:8443 --metrics-secure=true
```

When `--metrics-secure=true`, the endpoint is protected with Kubernetes authn/authz via controller-runtime `FilterProvider`.

## Custom Metrics

All custom metrics use the `sreportal_` prefix and are defined in `internal/metrics/metrics.go`.

### Controller Metrics

Reconciliation performance and error tracking across all controllers (dns, portal, alertmanager).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_controller_reconcile_total` | Counter | `controller`, `result` | Reconciliation count by result (`success`, `error`) |
| `sreportal_controller_reconcile_duration_seconds` | Histogram | `controller` | Reconciliation latency distribution |

### DNS Metrics

Track the volume of DNS data managed by the operator.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_dns_fqdns_total` | Gauge | `portal`, `source` | Number of FQDNs per portal and source (`manual`, `external-dns`, `remote`) |
| `sreportal_dns_groups_total` | Gauge | `portal` | Number of DNS groups per portal |

### Source Metrics

Monitor the external-dns source collection pipeline.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_source_endpoints_collected` | Gauge | `source_type` | Endpoints collected per source type (`service`, `ingress`, `dnsendpoint`, etc.) |
| `sreportal_source_errors_total` | Counter | `source_type` | Cumulative source collection errors |

### Alertmanager Metrics

Monitor alert fetching from Alertmanager instances.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_alertmanager_alerts_active` | Gauge | `portal`, `alertmanager` | Number of active alerts per Alertmanager resource |
| `sreportal_alertmanager_fetch_errors_total` | Counter | `alertmanager` | Cumulative alert fetch errors |

### Portal Metrics

Track portal inventory and remote synchronization health.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_portal_total` | Gauge | `type` | Number of portals by type (`local`, `remote`) |
| `sreportal_portal_remote_sync_errors_total` | Counter | `portal` | Cumulative remote portal sync errors |
| `sreportal_portal_remote_fqdns_synced` | Gauge | `portal` | FQDNs synced from each remote portal |

### HTTP Server Metrics

Request-level metrics for the web server (Connect API, MCP, static files).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_http_requests_total` | Counter | `method`, `handler`, `code` | HTTP requests by method, handler, and status code |
| `sreportal_http_request_duration_seconds` | Histogram | `method`, `handler` | HTTP request latency distribution |
| `sreportal_http_requests_in_flight` | Gauge | — | Number of HTTP requests currently being processed |

The `handler` label uses low-cardinality values: `connect` (gRPC/Connect API), `mcp` (MCP servers), `api` (health endpoints), `static` (web UI files).

### MCP Server Metrics

Track MCP tool usage and session activity.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_mcp_tool_calls_total` | Counter | `server`, `tool` | MCP tool invocations |
| `sreportal_mcp_tool_call_duration_seconds` | Histogram | `server`, `tool` | MCP tool call latency |
| `sreportal_mcp_tool_call_errors_total` | Counter | `server`, `tool` | MCP tool call errors |
| `sreportal_mcp_sessions_active` | Gauge | `server` | Active MCP sessions (`dns`, `alerts`) |

## Built-in Metrics

The `/metrics` endpoint also exposes standard metrics from controller-runtime and the Go runtime:

| Category | Examples |
|----------|----------|
| **Reconciliation** | `controller_runtime_reconcile_total`, `controller_runtime_reconcile_time_seconds` |
| **Work queue** | `workqueue_adds_total`, `workqueue_depth`, `workqueue_queue_duration_seconds` |
| **REST client** | `rest_client_requests_total`, `rest_client_request_duration_seconds` |
| **Leader election** | `leader_election_master_status` |
| **Go runtime** | `go_goroutines`, `go_memstats_*`, `go_gc_duration_seconds` |
| **Process** | `process_cpu_seconds_total`, `process_resident_memory_bytes`, `process_open_fds` |

## Grafana Dashboard

A pre-built Grafana dashboard is available at [`config/grafana/sreportal-dashboard.json`](https://github.com/golgoth31/sreportal/blob/main/config/grafana/sreportal-dashboard.json).

### Import

1. Open Grafana
2. Go to **Dashboards → Import**
3. Upload `config/grafana/sreportal-dashboard.json` or paste the JSON content
4. Select your Prometheus datasource

### Variables

The dashboard includes two template variables:

| Variable | Type | Description |
|----------|------|-------------|
| `datasource` | Datasource | Prometheus datasource picker — select from all available Prometheus datasources |
| `job` | Query | Prometheus job filter — auto-discovered from `sreportal_controller_reconcile_total`, multi-select with "All" |

### Dashboard Layout

The dashboard is organized into two rows:

#### Row 1 — Application Metrics

| Panel | Visualization | Content |
|-------|---------------|---------|
| **Reconciliations / sec** | Time series | Rate of `sreportal_controller_reconcile_total` by controller and result |
| **Reconciliation Duration** | Time series | p50 / p95 / p99 of `reconcile_duration_seconds` by controller |
| **FQDNs Total** | Stat | Sum of `sreportal_dns_fqdns_total` per portal |
| **DNS Groups** | Stat | Sum of `sreportal_dns_groups_total` per portal |
| **Active Alerts** | Stat | Sum of `sreportal_alertmanager_alerts_active` per portal/alertmanager (thresholds: green → orange → red) |
| **Remote FQDNs Synced** | Stat | `sreportal_portal_remote_fqdns_synced` per portal |
| **HTTP Requests / sec** | Time series (stacked) | Rate of `sreportal_http_requests_total` by handler and status code |
| **HTTP Latency** | Time series | p50 / p95 / p99 of `http_request_duration_seconds` by handler |
| **HTTP In-Flight / MCP Sessions** | Time series | `requests_in_flight` and `mcp_sessions_active` |
| **MCP Tool Calls / sec** | Time series (bars) | Rate of `mcp_tool_calls_total` by server/tool |
| **Source Endpoints Collected** | Time series | `source_endpoints_collected` by source type |
| **Errors / sec** | Time series | Combined error rates: source, alertmanager fetch, remote sync, MCP tool errors |
| **Portals** | Gauge | `sreportal_portal_total` by type (local / remote) |

#### Row 2 — System Metrics

| Panel | Visualization | Content |
|-------|---------------|---------|
| **CPU Usage** | Time series | `rate(process_cpu_seconds_total)` |
| **Memory Usage** | Time series | RSS, heap alloc, heap in-use, stack in-use |
| **Goroutines** | Time series | `go_goroutines` |
| **Open File Descriptors** | Time series | `process_open_fds` vs `process_max_fds` |
| **GC Duration** | Time series | `go_gc_duration_seconds` p50 / p75 / p100 |
| **K8s REST Client Requests / sec** | Time series (stacked) | `rest_client_requests_total` by method and status code |
| **Workqueue Depth** | Time series | `workqueue_depth` per controller queue |

### Provisioning

To auto-provision the dashboard via Grafana's provisioning system, add it to your Grafana provisioning configuration:

```yaml
# grafana/provisioning/dashboards/sreportal.yaml
apiVersion: 1
providers:
  - name: sreportal
    type: file
    options:
      path: /var/lib/grafana/dashboards/sreportal
```

Then mount or copy `config/grafana/sreportal-dashboard.json` into the configured path.

## Prometheus Scrape Configuration

Example ServiceMonitor for Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: sreportal
  namespace: sreportal-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: sreportal
  endpoints:
    - port: metrics
      scheme: https
      tlsConfig:
        insecureSkipVerify: true
      bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
```
