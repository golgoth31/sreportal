/**
 * Dev-only MSW handlers — emit realistic gRPC-Web responses so the UI
 * can be exercised without a backend. Activated via `VITE_MOCK=1`.
 */
import { create, toBinary, type DescMessage, type MessageShape } from "@bufbuild/protobuf";
import { http, HttpResponse } from "msw";

import {
  FQDNSchema,
  ListFQDNsResponseSchema,
} from "@/gen/sreportal/v1/dns_pb";
import {
  ListPortalsResponseSchema,
  PortalSchema,
} from "@/gen/sreportal/v1/portal_pb";
import {
  AlertSchema,
  AlertmanagerResourceSchema,
  ListAlertsResponseSchema,
} from "@/gen/sreportal/v1/alertmanager_pb";
import {
  ListMetricsResponseSchema,
  MetricFamilySchema,
  MetricSchema,
} from "@/gen/sreportal/v1/metrics_pb";
import { GetVersionResponseSchema } from "@/gen/sreportal/v1/version_pb";

const GRPC_WEB_HEADERS = { "Content-Type": "application/grpc-web+proto" };

function frame<T extends DescMessage>(schema: T, message: MessageShape<T>) {
  const data = toBinary(schema, message);
  const trailers = new TextEncoder().encode("grpc-status:0\r\n");
  const buf = new Uint8Array(1 + 4 + data.length + 1 + 4 + trailers.length);
  const view = new DataView(buf.buffer);
  let o = 0;
  buf[o++] = 0x00;
  view.setUint32(o, data.length);
  o += 4;
  buf.set(data, o);
  o += data.length;
  buf[o++] = 0x80;
  view.setUint32(o, trailers.length);
  o += 4;
  buf.set(trailers, o);
  return new HttpResponse(buf, { headers: GRPC_WEB_HEADERS });
}

// ---------- Portals --------------------------------------------------------

const PORTALS = [
  create(PortalSchema, {
    name: "main",
    title: "Main",
    main: true,
    namespace: "default",
    ready: true,
    features: { dns: true, networkPolicy: true, statusPage: true, alerts: true },
  }),
  create(PortalSchema, {
    name: "prod-us",
    title: "Production US",
    subPath: "prod-us",
    namespace: "platform",
    ready: true,
    features: { dns: true, networkPolicy: true, statusPage: true, alerts: true, imageInventory: true, releases: true },
  }),
  create(PortalSchema, {
    name: "prod-eu",
    title: "Production EU",
    subPath: "prod-eu",
    namespace: "platform",
    ready: true,
    features: { dns: true, networkPolicy: true, alerts: true },
  }),
  create(PortalSchema, {
    name: "staging",
    title: "Staging",
    subPath: "staging",
    namespace: "staging",
    ready: true,
    features: { dns: true, networkPolicy: true, alerts: true },
  }),
  create(PortalSchema, {
    name: "customer-acme",
    title: "Acme",
    subPath: "customer-acme",
    namespace: "customers",
    ready: true,
    isRemote: true,
    url: "https://acme.portal.example.com",
    features: { dns: true, alerts: true },
  }),
  create(PortalSchema, {
    name: "customer-northwind",
    title: "Northwind",
    subPath: "customer-northwind",
    namespace: "customers",
    ready: true,
    isRemote: true,
    url: "https://northwind.portal.example.com",
    features: { dns: true },
  }),
];

// ---------- FQDNs ----------------------------------------------------------

const fqdn = (
  name: string,
  groups: string[],
  description: string,
  syncStatus: "sync" | "outofsync" | "notavailable",
  source: "manual" | "external-dns" = "external-dns",
  recordType = "A",
  targets: string[] = [],
) =>
  create(FQDNSchema, {
    name,
    groups,
    description,
    syncStatus,
    source,
    recordType,
    targets,
    dnsResourceName: "dns-sample",
    dnsResourceNamespace: "default",
  });

const FQDNS = [
  fqdn("api.public.acme.io", ["api.public"], "Public REST API gateway", "sync", "external-dns", "A", ["34.107.224.12"]),
  fqdn("checkout.api.public.acme.io", ["api.public"], "Checkout sub-domain", "sync", "external-dns", "A", ["34.107.224.18"]),
  fqdn("orders.api.public.acme.io", ["api.public"], "Orders service", "sync", "external-dns", "A", ["34.107.224.21"]),
  fqdn("billing.public.acme.io", ["api.public"], "Billing CNAME (drifting)", "outofsync", "manual", "CNAME", ["billing-lb.elb.amazonaws.com"]),
  fqdn("search.api.public.acme.io", ["api.public"], "Search service", "sync", "external-dns", "A", ["34.107.224.27"]),
  fqdn("grafana.infra.acme.io", ["infra.observability"], "Grafana dashboards", "sync", "external-dns", "A", ["10.20.4.81"]),
  fqdn("prometheus.infra.acme.io", ["infra.observability"], "Prometheus metrics", "sync", "external-dns", "A", ["10.20.4.82"]),
  fqdn("tempo.infra.acme.io", ["infra.observability"], "Tempo traces", "sync", "external-dns", "A", ["10.20.4.83"]),
  fqdn("loki.infra.acme.io", ["infra.observability"], "Loki logs", "sync", "external-dns", "A", ["10.20.4.84"]),
  fqdn("alertmanager.infra.acme.io", ["infra.observability"], "Alertmanager UI", "sync", "external-dns", "A", ["10.20.4.85"]),
  fqdn("argocd.infra.acme.io", ["infra.observability"], "ArgoCD GitOps", "sync", "external-dns", "A", ["10.20.4.94"]),
  fqdn("dashboard.web.acme.io", ["web.dashboard"], "Customer web dashboard", "sync", "external-dns", "A", ["34.107.224.50"]),
  fqdn("admin.web.acme.io", ["web.dashboard"], "Internal admin dashboard", "sync", "external-dns", "A", ["10.20.4.50"]),
  fqdn("legacy-v1.acme.io", ["legacy.v1"], "Legacy v1 endpoint (to be retired)", "notavailable", "manual", "A", ["198.51.100.42"]),
];

// ---------- Alerts ---------------------------------------------------------

const al = (
  fingerprint: string,
  alertname: string,
  state: "active" | "suppressed",
  startsAt: Date,
  summary: string,
  instance = "checkout-api-7d4f-x9k2",
  silenced = false,
) =>
  create(AlertSchema, {
    fingerprint,
    state,
    startsAt: { seconds: BigInt(Math.floor(startsAt.getTime() / 1000)), nanos: 0 },
    annotations: { summary, description: summary },
    labels: { alertname, severity: state === "active" ? "critical" : "warning", instance },
    receivers: ["slack-sre", "pagerduty"],
    silencedBy: silenced ? ["silence-1"] : [],
  });

const NOW = Date.now();
const ALERTS = [
  al("a1", "HighMemoryUsage", "active", new Date(NOW - 2 * 60_000),
    "Pod checkout-api-7d4f-x9k2 is using 94% of its 2 GiB memory limit"),
  al("a2", "DNSResolutionFailing", "active", new Date(NOW - 7 * 60_000),
    "billing.public.acme.io is returning NXDOMAIN (×42 in the last 5m)", "dns-controller-1"),
  al("a3", "CertificateExpiringSoon", "active", new Date(NOW - 14 * 60_000),
    "Wildcard *.staging.acme.io expires in 7 days", "cert-manager"),
  al("a4", "SlowQueryDetected", "active", new Date(NOW - 23 * 60_000),
    "cloudsql/orders-prod-us p95 = 4.2s (threshold 1s)", "orders-prod-us"),
  al("a5", "NodeNotReady", "suppressed", new Date(NOW - 60 * 60_000),
    "gke-prod-us-1-default-pool-3a marked NotReady (maintenance window)", "gke-prod-us-1-default-pool-3a", true),
];

const ALERT_RESOURCES = [
  create(AlertmanagerResourceSchema, {
    name: "alertmanager-prod",
    namespace: "monitoring",
    portalRef: "main",
    ready: true,
    localUrl: "http://alertmanager.monitoring.svc.cluster.local:9093",
    remoteUrl: "https://alertmanager.infra.acme.io",
    alerts: ALERTS,
  }),
];

// ---------- Metrics --------------------------------------------------------

function family(
  name: string,
  help: string,
  type: "GAUGE" | "COUNTER",
  samples: Array<[Record<string, string>, number]>,
) {
  return create(MetricFamilySchema, {
    name,
    help,
    type,
    metrics: samples.map(([labels, value]) =>
      create(MetricSchema, { labels, value }),
    ),
  });
}
const gauge = (n: string, h: string, s: Array<[Record<string, string>, number]>) =>
  family(n, h, "GAUGE", s);
const counter = (n: string, h: string, s: Array<[Record<string, string>, number]>) =>
  family(n, h, "COUNTER", s);

const METRIC_FAMILIES = [
  gauge("sreportal_dns_fqdns_total", "Total FQDNs per portal/group", [
    [{ portal: "main", group: "api.public" }, 142],
    [{ portal: "main", group: "infra.observability" }, 38],
    [{ portal: "main", group: "web.dashboard" }, 74],
    [{ portal: "main", group: "legacy.v1" }, 28],
    [{ portal: "prod-us", group: "api.public" }, 412],
    [{ portal: "prod-us", group: "data.pipelines" }, 142],
    [{ portal: "prod-eu", group: "api.public" }, 318],
    [{ portal: "staging", group: "web.dashboard" }, 84],
  ]),
  gauge("sreportal_alertmanager_alerts_active", "Active alerts per portal", [
    [{ portal: "main", severity: "critical" }, 4],
    [{ portal: "main", severity: "warning" }, 3],
  ]),
  gauge("sreportal_portal_total", "Number of configured portals", [
    [{ type: "local" }, 4],
    [{ type: "remote" }, 2],
  ]),
  gauge("sreportal_http_requests_in_flight", "In-flight HTTP requests", [
    [{}, 23],
  ]),
  gauge("sreportal_mcp_sessions_active", "Active MCP streamable sessions", [
    [{ client: "claude" }, 7],
    [{ client: "cursor" }, 4],
    [{ client: "codex" }, 3],
  ]),
  counter("sreportal_source_endpoints_collected", "DNS source endpoints discovered", [
    [{ source: "external-dns" }, 380],
    [{ source: "manual" }, 32],
  ]),
];

// ---------- Handlers -------------------------------------------------------

const re = (suffix: string) => new RegExp(`/sreportal\\.v1\\.[A-Za-z]+/${suffix}$`);

export const devHandlers = [
  http.post(re("GetVersion"), () =>
    frame(
      GetVersionResponseSchema,
      create(GetVersionResponseSchema, {
        version: "1.30.0-mock",
        commit: "mock",
        date: "2026-04-30",
      }),
    ),
  ),
  http.post(re("ListPortals"), () =>
    frame(ListPortalsResponseSchema, create(ListPortalsResponseSchema, { portals: PORTALS })),
  ),
  http.post(re("ListFQDNs"), () =>
    frame(
      ListFQDNsResponseSchema,
      create(ListFQDNsResponseSchema, {
        fqdns: FQDNS,
        nextPageToken: "",
        totalSize: FQDNS.length,
      }),
    ),
  ),
  http.post(re("ListAlerts"), () =>
    frame(
      ListAlertsResponseSchema,
      create(ListAlertsResponseSchema, { alertmanagers: ALERT_RESOURCES }),
    ),
  ),
  http.post(re("ListMetrics"), () =>
    frame(
      ListMetricsResponseSchema,
      create(ListMetricsResponseSchema, { families: METRIC_FAMILIES }),
    ),
  ),
];

