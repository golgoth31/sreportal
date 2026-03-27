#!/usr/bin/env npx tsx
/**
 * Generate PNG screenshots for every page of the SRE Portal web UI
 * using Playwright with mocked gRPC-web API responses (proper protobuf serialization).
 *
 * Usage: npx tsx scripts/screenshots.ts
 * Output: screenshots/*.png
 */

import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { create, toBinary, type MessageShape, type DescMessage } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  ListPortalsResponseSchema,
  PortalSchema,
  RemoteSyncStatusSchema,
} from "../src/gen/sreportal/v1/portal_pb.js";
import {
  ListFQDNsResponseSchema,
  FQDNSchema,
} from "../src/gen/sreportal/v1/dns_pb.js";
import {
  ListAlertsResponseSchema,
  AlertmanagerResourceSchema,
  AlertSchema,
} from "../src/gen/sreportal/v1/alertmanager_pb.js";
import {
  ListReleasesResponseSchema,
  ListReleaseDaysResponseSchema,
  ReleaseEntrySchema,
  ReleaseTypeConfigSchema,
} from "../src/gen/sreportal/v1/release_pb.js";
import {
  ListNetworkPoliciesResponseSchema,
  NetpolNodeSchema,
  NetpolEdgeSchema,
} from "../src/gen/sreportal/v1/netpol_pb.js";
import {
  ListMetricsResponseSchema,
  MetricFamilySchema,
  MetricSchema,
  HistogramValueSchema,
  HistogramBucketSchema,
} from "../src/gen/sreportal/v1/metrics_pb.js";
import {
  GetVersionResponseSchema,
} from "../src/gen/sreportal/v1/version_pb.js";
import {
  ListComponentsResponseSchema,
  ListMaintenancesResponseSchema,
  ListIncidentsResponseSchema,
  ComponentResourceSchema,
  MaintenanceResourceSchema,
  IncidentResourceSchema,
  IncidentUpdateSchema,
  ComponentStatus,
  MaintenancePhase,
  IncidentPhase,
  IncidentSeverity,
} from "../src/gen/sreportal/v1/status_pb.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");
const IMG_BASE = path.resolve(ROOT, "..", "docs", "content", "assets", "img");

// ── gRPC-web envelope framing ──────────────────────────────────────────

function grpcWebFrame(schema: DescMessage, msg: MessageShape<DescMessage>): Buffer {
  const data = toBinary(schema, msg);
  // Data frame: flag=0x00, 4-byte BE length, payload
  const dataFrame = Buffer.alloc(5 + data.length);
  dataFrame[0] = 0x00;
  dataFrame.writeUInt32BE(data.length, 1);
  Buffer.from(data).copy(dataFrame, 5);

  // Trailer frame: flag=0x80, 4-byte BE length, "grpc-status:0\r\n"
  const trailer = Buffer.from("grpc-status:0\r\n");
  const trailerFrame = Buffer.alloc(5 + trailer.length);
  trailerFrame[0] = 0x80;
  trailerFrame.writeUInt32BE(trailer.length, 1);
  trailer.copy(trailerFrame, 5);

  return Buffer.concat([dataFrame, trailerFrame]);
}

// ── Build mock proto messages ──────────────────────────────────────────

const now = new Date();
function ago(seconds: number): Date {
  return new Date(now.getTime() - seconds * 1000);
}

// Portals
const portalsResponse = create(ListPortalsResponseSchema, {
  portals: [
    create(PortalSchema, {
      name: "main", title: "Main Portal", main: true, subPath: "",
      namespace: "default", ready: true, url: "", isRemote: false,
    }),
    create(PortalSchema, {
      name: "staging", title: "Staging", main: false, subPath: "staging",
      namespace: "staging", ready: true, url: "https://staging.example.com",
      isRemote: true,
      remoteSync: create(RemoteSyncStatusSchema, {
        lastSyncTime: "2026-03-24T10:00:00Z", lastSyncError: "",
        remoteTitle: "Staging Portal", fqdnCount: 12,
      }),
    }),
  ],
});

// FQDNs
const fqdnsResponse = create(ListFQDNsResponseSchema, {
  fqdns: [
    create(FQDNSchema, { name: "api.example.com", source: "external-dns", groups: ["backend"], description: "Main API endpoint", recordType: "A", targets: ["10.0.1.10"], dnsResourceName: "api-dns", dnsResourceNamespace: "default", syncStatus: "sync" }),
    create(FQDNSchema, { name: "web.example.com", source: "external-dns", groups: ["frontend"], description: "Web application", recordType: "CNAME", targets: ["cdn.example.com"], dnsResourceName: "web-dns", dnsResourceNamespace: "default", syncStatus: "sync" }),
    create(FQDNSchema, { name: "auth.example.com", source: "external-dns", groups: ["backend"], description: "Auth service", recordType: "A", targets: ["10.0.1.20"], dnsResourceName: "auth-dns", dnsResourceNamespace: "default", syncStatus: "notsync" }),
    create(FQDNSchema, { name: "grafana.internal.io", source: "manual", groups: ["monitoring"], description: "Grafana dashboard", recordType: "A", targets: ["10.0.2.10"], dnsResourceName: "grafana-dns", dnsResourceNamespace: "monitoring", syncStatus: "sync" }),
    create(FQDNSchema, { name: "prometheus.internal.io", source: "manual", groups: ["monitoring"], description: "Prometheus server", recordType: "A", targets: ["10.0.2.11"], dnsResourceName: "prom-dns", dnsResourceNamespace: "monitoring", syncStatus: "sync" }),
    create(FQDNSchema, { name: "loki.internal.io", source: "manual", groups: ["monitoring"], description: "Loki log aggregation", recordType: "A", targets: ["10.0.2.12"], dnsResourceName: "loki-dns", dnsResourceNamespace: "monitoring", syncStatus: "notavailable" }),
    create(FQDNSchema, { name: "argocd.platform.io", source: "external-dns", groups: ["platform"], description: "ArgoCD dashboard", recordType: "A", targets: ["10.0.3.1"], dnsResourceName: "argocd-dns", dnsResourceNamespace: "platform", syncStatus: "sync" }),
    create(FQDNSchema, { name: "vault.platform.io", source: "external-dns", groups: ["platform", "security"], description: "HashiCorp Vault", recordType: "A", targets: ["10.0.3.2"], dnsResourceName: "vault-dns", dnsResourceNamespace: "platform", syncStatus: "sync" }),
    create(FQDNSchema, { name: "keycloak.auth.io", source: "external-dns", groups: ["security"], description: "Keycloak SSO", recordType: "A", targets: ["10.0.4.1"], dnsResourceName: "kc-dns", dnsResourceNamespace: "auth", syncStatus: "sync" }),
    create(FQDNSchema, { name: "rabbitmq.messaging.io", source: "external-dns", groups: ["backend"], description: "RabbitMQ cluster", recordType: "A", targets: ["10.0.5.1", "10.0.5.2"], dnsResourceName: "rmq-dns", dnsResourceNamespace: "messaging", syncStatus: "sync" }),
  ],
  totalSize: 10,
});

// Alerts
const alertsResponse = create(ListAlertsResponseSchema, {
  alertmanagers: [
    create(AlertmanagerResourceSchema, {
      name: "prod-alertmanager", namespace: "monitoring", portalRef: "main",
      localUrl: "http://alertmanager.monitoring:9093", remoteUrl: "", ready: true,
      lastReconcileTime: timestampFromDate(ago(60)),
      alerts: [
        create(AlertSchema, { fingerprint: "abc123", labels: { alertname: "HighCPU", severity: "critical", namespace: "backend", service: "api-server" }, annotations: { summary: "CPU usage above 90% for 10 minutes" }, state: "active", startsAt: timestampFromDate(ago(3600)), updatedAt: timestampFromDate(ago(120)), receivers: ["slack-critical"] }),
        create(AlertSchema, { fingerprint: "def456", labels: { alertname: "PodCrashLooping", severity: "warning", namespace: "payments", pod: "payments-worker-7f8d9" }, annotations: { summary: "Pod restarted 5 times in last 15 minutes" }, state: "active", startsAt: timestampFromDate(ago(900)), updatedAt: timestampFromDate(ago(60)), receivers: ["slack-warnings"] }),
        create(AlertSchema, { fingerprint: "ghi789", labels: { alertname: "HighMemory", severity: "warning", namespace: "backend", service: "auth-service" }, annotations: { summary: "Memory usage above 85%" }, state: "active", startsAt: timestampFromDate(ago(7200)), updatedAt: timestampFromDate(ago(300)), receivers: ["slack-warnings"] }),
        create(AlertSchema, { fingerprint: "jkl012", labels: { alertname: "CertExpiringSoon", severity: "info", namespace: "ingress", domain: "api.example.com" }, annotations: { summary: "TLS certificate expires in 7 days" }, state: "suppressed", startsAt: timestampFromDate(ago(86400)), updatedAt: timestampFromDate(ago(3600)), receivers: ["email-ops"], silencedBy: ["silence-1"] }),
      ],
    }),
  ],
});

// Releases
const releasesResponse = create(ListReleasesResponseSchema, {
  day: "2026-03-24",
  previousDay: "2026-03-23",
  nextDay: "",
  entries: [
    create(ReleaseEntrySchema, { type: "release", version: "v2.14.0", origin: "api-server", date: timestampFromDate(ago(7200)), author: "alice", message: "Add new payment provider integration", link: "https://github.com/example/api/releases/v2.14.0" }),
    create(ReleaseEntrySchema, { type: "release", version: "v1.8.3", origin: "web-frontend", date: timestampFromDate(ago(5400)), author: "bob", message: "Fix responsive layout on mobile devices" }),
    create(ReleaseEntrySchema, { type: "hotfix", version: "v2.13.1", origin: "api-server", date: timestampFromDate(ago(3600)), author: "alice", message: "Fix race condition in payment processing", link: "https://github.com/example/api/releases/v2.13.1" }),
    create(ReleaseEntrySchema, { type: "config", origin: "platform/helm-values", date: timestampFromDate(ago(1800)), author: "charlie", message: "Increase replica count for api-server to 5" }),
    create(ReleaseEntrySchema, { type: "rollback", version: "v1.8.2", origin: "web-frontend", date: timestampFromDate(ago(900)), author: "bob", message: "Rollback due to broken build pipeline" }),
    create(ReleaseEntrySchema, { type: "canary", version: "v2.15.0-rc1", origin: "api-server", date: timestampFromDate(ago(600)), author: "alice", message: "Canary deploy for new search endpoint" }),
  ],
});

const releaseDaysResponse = create(ListReleaseDaysResponseSchema, {
  days: ["2026-03-22", "2026-03-23", "2026-03-24"],
  ttlDays: 30,
  types: [
    create(ReleaseTypeConfigSchema, { name: "release", color: "#3b82f6" }),
    create(ReleaseTypeConfigSchema, { name: "rollback", color: "#f97316" }),
    create(ReleaseTypeConfigSchema, { name: "hotfix", color: "#ef4444" }),
    create(ReleaseTypeConfigSchema, { name: "canary", color: "#eab308" }),
    create(ReleaseTypeConfigSchema, { name: "config", color: "#14b8a6" }),
  ],
});

// Network Policies
const netpolResponse = create(ListNetworkPoliciesResponseSchema, {
  nodes: [
    create(NetpolNodeSchema, { id: "backend/api-server", label: "api-server", namespace: "backend", nodeType: "service", group: "backend" }),
    create(NetpolNodeSchema, { id: "backend/auth-service", label: "auth-service", namespace: "backend", nodeType: "service", group: "backend" }),
    create(NetpolNodeSchema, { id: "payments/payments-worker", label: "payments-worker", namespace: "payments", nodeType: "service", group: "payments" }),
    create(NetpolNodeSchema, { id: "frontend/web-app", label: "web-app", namespace: "frontend", nodeType: "service", group: "frontend" }),
    create(NetpolNodeSchema, { id: "monitoring/prometheus", label: "prometheus", namespace: "monitoring", nodeType: "service", group: "monitoring" }),
    create(NetpolNodeSchema, { id: "monitoring/grafana", label: "grafana", namespace: "monitoring", nodeType: "service", group: "monitoring" }),
    create(NetpolNodeSchema, { id: "platform/argocd", label: "argocd", namespace: "platform", nodeType: "service", group: "platform" }),
    create(NetpolNodeSchema, { id: "external/stripe-api", label: "stripe-api", namespace: "external", nodeType: "fqdn", group: "external" }),
  ],
  edges: [
    create(NetpolEdgeSchema, { from: "frontend/web-app", to: "backend/api-server", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "backend/api-server", to: "backend/auth-service", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "backend/api-server", to: "payments/payments-worker", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "payments/payments-worker", to: "external/stripe-api", edgeType: "fqdn" }),
    create(NetpolEdgeSchema, { from: "monitoring/prometheus", to: "backend/api-server", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "monitoring/prometheus", to: "backend/auth-service", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "monitoring/prometheus", to: "payments/payments-worker", edgeType: "ingress" }),
    create(NetpolEdgeSchema, { from: "monitoring/grafana", to: "monitoring/prometheus", edgeType: "ingress" }),
  ],
});

// Metrics
const metricsResponse = create(ListMetricsResponseSchema, {
  families: [
    create(MetricFamilySchema, { name: "sreportal_dns_fqdns_total", help: "Total number of FQDNs discovered", type: "GAUGE", metrics: [
      create(MetricSchema, { labels: { portal: "main", source: "external-dns" }, value: 7 }),
      create(MetricSchema, { labels: { portal: "main", source: "manual" }, value: 3 }),
      create(MetricSchema, { labels: { portal: "staging", source: "external-dns" }, value: 12 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_alertmanager_alerts_active", help: "Number of active alerts", type: "GAUGE", metrics: [
      create(MetricSchema, { labels: { portal: "main" }, value: 3 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_portal_total", help: "Number of portals", type: "GAUGE", metrics: [
      create(MetricSchema, { labels: { type: "local" }, value: 1 }),
      create(MetricSchema, { labels: { type: "remote" }, value: 1 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_http_requests_in_flight", help: "Number of HTTP requests currently being served", type: "GAUGE", metrics: [
      create(MetricSchema, { value: 4 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_mcp_sessions_active", help: "Number of active MCP sessions", type: "GAUGE", metrics: [
      create(MetricSchema, { value: 2 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_source_endpoints_collected", help: "Number of source endpoints collected", type: "GAUGE", metrics: [
      create(MetricSchema, { value: 48 }),
    ]}),
    create(MetricFamilySchema, { name: "sreportal_http_request_duration_seconds", help: "Duration of HTTP requests in seconds", type: "HISTOGRAM", metrics: [
      create(MetricSchema, { labels: { method: "GET", path: "/api/v1/fqdns" }, value: 0, histogram: create(HistogramValueSchema, {
        sampleCount: BigInt(1240), sampleSum: 62.5, buckets: [
          create(HistogramBucketSchema, { cumulativeCount: BigInt(800), upperBound: 0.01 }),
          create(HistogramBucketSchema, { cumulativeCount: BigInt(1100), upperBound: 0.05 }),
          create(HistogramBucketSchema, { cumulativeCount: BigInt(1200), upperBound: 0.1 }),
          create(HistogramBucketSchema, { cumulativeCount: BigInt(1235), upperBound: 0.5 }),
          create(HistogramBucketSchema, { cumulativeCount: BigInt(1240), upperBound: 1.0 }),
        ],
      })}),
    ]}),
  ],
});

// Version
const versionResponse = create(GetVersionResponseSchema, {
  version: "1.20.0", commit: "5b58fdd", date: "2026-03-24T08:00:00Z",
});

// Status Page — Components
const componentsResponse = create(ListComponentsResponseSchema, {
  components: [
    create(ComponentResourceSchema, { name: "api-server", displayName: "API Server", description: "Main REST API", group: "Backend", link: "https://api.example.com", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(86400)) }),
    create(ComponentResourceSchema, { name: "auth-service", displayName: "Auth Service", description: "Authentication & SSO", group: "Backend", link: "", portalRef: "main", declaredStatus: ComponentStatus.DEGRADED, computedStatus: ComponentStatus.DEGRADED, activeIncidents: 1, lastStatusChange: timestampFromDate(ago(1800)) }),
    create(ComponentResourceSchema, { name: "payments-worker", displayName: "Payments Worker", description: "Async payment processing", group: "Backend", link: "", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(172800)) }),
    create(ComponentResourceSchema, { name: "web-frontend", displayName: "Web Frontend", description: "React SPA", group: "Frontend", link: "https://web.example.com", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(43200)) }),
    create(ComponentResourceSchema, { name: "cdn", displayName: "CDN", description: "Static assets delivery", group: "Frontend", link: "", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(604800)) }),
    create(ComponentResourceSchema, { name: "postgres-primary", displayName: "PostgreSQL Primary", description: "Primary database", group: "Data", link: "", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.MAINTENANCE, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(600)) }),
    create(ComponentResourceSchema, { name: "redis-cluster", displayName: "Redis Cluster", description: "Cache and sessions", group: "Data", link: "", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(259200)) }),
    create(ComponentResourceSchema, { name: "rabbitmq", displayName: "RabbitMQ", description: "Message broker", group: "Data", link: "", portalRef: "main", declaredStatus: ComponentStatus.OPERATIONAL, computedStatus: ComponentStatus.OPERATIONAL, activeIncidents: 0, lastStatusChange: timestampFromDate(ago(432000)) }),
  ],
});

// Status Page — Maintenances
const maintenancesResponse = create(ListMaintenancesResponseSchema, {
  maintenances: [
    create(MaintenanceResourceSchema, { name: "db-upgrade", title: "PostgreSQL 16 Upgrade", description: "Upgrading primary database from PostgreSQL 15 to 16. Expect brief read-only period.", portalRef: "main", components: ["postgres-primary"], scheduledStart: timestampFromDate(ago(600)), scheduledEnd: timestampFromDate(new Date(now.getTime() + 3600 * 1000)), affectedStatus: "maintenance", phase: MaintenancePhase.IN_PROGRESS }),
    create(MaintenanceResourceSchema, { name: "network-migration", title: "Network Subnet Migration", description: "Migrating backend services to new subnet range for improved isolation.", portalRef: "main", components: ["api-server", "auth-service", "payments-worker"], scheduledStart: timestampFromDate(new Date(now.getTime() + 86400 * 1000)), scheduledEnd: timestampFromDate(new Date(now.getTime() + 90000 * 1000)), affectedStatus: "maintenance", phase: MaintenancePhase.UPCOMING }),
  ],
});

// Status Page — Incidents
const incidentsResponse = create(ListIncidentsResponseSchema, {
  incidents: [
    create(IncidentResourceSchema, {
      name: "auth-latency", title: "Elevated Auth Latency", portalRef: "main",
      components: ["auth-service"], severity: IncidentSeverity.MAJOR,
      currentPhase: IncidentPhase.IDENTIFIED, startedAt: timestampFromDate(ago(1800)),
      resolvedAt: undefined, durationMinutes: 0,
      updates: [
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(1800)), phase: IncidentPhase.INVESTIGATING, message: "Monitoring detected elevated p99 latency on auth endpoints." }),
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(1200)), phase: IncidentPhase.IDENTIFIED, message: "Root cause identified: connection pool exhaustion on auth-service replica set." }),
      ],
    }),
    create(IncidentResourceSchema, {
      name: "cdn-outage", title: "CDN Cache Invalidation Failure", portalRef: "main",
      components: ["cdn", "web-frontend"], severity: IncidentSeverity.MINOR,
      currentPhase: IncidentPhase.RESOLVED, startedAt: timestampFromDate(ago(7200)),
      resolvedAt: timestampFromDate(ago(3600)), durationMinutes: 60,
      updates: [
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(7200)), phase: IncidentPhase.INVESTIGATING, message: "Reports of stale content being served from CDN edge nodes." }),
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(5400)), phase: IncidentPhase.IDENTIFIED, message: "Cache invalidation API returned 503 due to upstream provider issue." }),
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(4200)), phase: IncidentPhase.MONITORING, message: "Provider confirmed fix deployed. Monitoring cache freshness." }),
        create(IncidentUpdateSchema, { timestamp: timestampFromDate(ago(3600)), phase: IncidentPhase.RESOLVED, message: "All edge nodes serving fresh content. Incident resolved." }),
      ],
    }),
  ],
});

// ── Route matching ─────────────────────────────────────────────────────

const API_ROUTES: Array<{ pattern: RegExp; schema: DescMessage; msg: MessageShape<DescMessage> }> = [
  { pattern: /PortalService\/ListPortals/, schema: ListPortalsResponseSchema, msg: portalsResponse },
  { pattern: /DNSService\/ListFQDNs/, schema: ListFQDNsResponseSchema, msg: fqdnsResponse },
  { pattern: /AlertmanagerService\/ListAlerts/, schema: ListAlertsResponseSchema, msg: alertsResponse },
  { pattern: /ReleaseService\/ListReleases$/, schema: ListReleasesResponseSchema, msg: releasesResponse },
  { pattern: /ReleaseService\/ListReleaseDays/, schema: ListReleaseDaysResponseSchema, msg: releaseDaysResponse },
  { pattern: /NetworkPolicyService\/ListNetworkPolicies/, schema: ListNetworkPoliciesResponseSchema, msg: netpolResponse },
  { pattern: /MetricsService\/ListMetrics/, schema: ListMetricsResponseSchema, msg: metricsResponse },
  { pattern: /VersionService\/GetVersion/, schema: GetVersionResponseSchema, msg: versionResponse },
  { pattern: /StatusService\/ListComponents/, schema: ListComponentsResponseSchema, msg: componentsResponse },
  { pattern: /StatusService\/ListMaintenances/, schema: ListMaintenancesResponseSchema, msg: maintenancesResponse },
  { pattern: /StatusService\/ListIncidents/, schema: ListIncidentsResponseSchema, msg: incidentsResponse },
];

// ── Pages to screenshot ────────────────────────────────────────────────

const PAGES = [
  { name: "links", path: "/main/links", waitFor: "text=api.example.com" },
  { name: "alerts", path: "/main/alerts", waitFor: "text=HighCPU" },
  { name: "dashboard", path: "/main/dashboard", waitFor: "text=Portal Statistics" },
  { name: "releases", path: "/main/releases", waitFor: "text=Releases" },
  { name: "netpol", path: "/main/netpol", waitFor: "text=Network Policies" },
  { name: "status", path: "/main/status", waitFor: "text=API Server" },
];

// ── Theme variants ─────────────────────────────────────────────────────

const THEMES: Array<{ name: string; colorScheme: "light" | "dark" }> = [
  { name: "light", colorScheme: "light" },
  { name: "dark", colorScheme: "dark" },
];

// ── Main ───────────────────────────────────────────────────────────────

async function main() {
  for (const theme of THEMES) {
    mkdirSync(path.join(IMG_BASE, theme.name), { recursive: true });
  }

  console.log("Starting Vite dev server...");
  const vite = spawn("npx", ["vite", "--port", "5199", "--strictPort"], {
    cwd: ROOT,
    stdio: ["ignore", "pipe", "pipe"],
    env: { ...process.env, BROWSER: "none" },
  });

  const baseUrl = await new Promise<string>((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error("Vite did not start within 30s")), 30_000);
    let output = "";
    vite.stdout!.on("data", (chunk: Buffer) => {
      output += chunk.toString();
      const match = output.match(/Local:\s+(http:\/\/localhost:\d+)/);
      if (match) {
        clearTimeout(timeout);
        resolve(match[1]);
      }
    });
    vite.stderr!.on("data", (chunk: Buffer) => { output += chunk.toString(); });
    vite.on("exit", (code) => {
      clearTimeout(timeout);
      reject(new Error(`Vite exited with code ${code}\n${output}`));
    });
  });

  console.log(`Vite running at ${baseUrl}`);

  try {
    const browser = await chromium.launch();

    for (const theme of THEMES) {
      console.log(`\n[${theme.name}] Capturing pages...`);
      const context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 2,
        colorScheme: theme.colorScheme,
      });

      const page = await context.newPage();

      // Intercept gRPC-web requests and return properly framed protobuf responses
      await page.route("**/*", (route) => {
        const url = route.request().url();
        const matched = API_ROUTES.find((r) => r.pattern.test(url));
        if (matched) {
          const body = grpcWebFrame(matched.schema, matched.msg);
          return route.fulfill({
            status: 200,
            headers: { "content-type": "application/grpc-web+proto" },
            body,
          });
        }
        return route.continue();
      });

      for (const { name, path: pagePath, waitFor } of PAGES) {
        console.log(`  Capturing ${name}...`);
        await page.goto(`${baseUrl}${pagePath}`, { waitUntil: "networkidle" });

        // Force theme class on <html> (app uses "system" by default via prefers-color-scheme)
        if (theme.colorScheme === "dark") {
          await page.evaluate(() => document.documentElement.classList.add("dark"));
        } else {
          await page.evaluate(() => document.documentElement.classList.remove("dark"));
        }

        try {
          await page.waitForSelector(waitFor, { timeout: 10_000 });
        } catch {
          console.warn(`    Warning: "${waitFor}" not found, taking screenshot anyway`);
        }

        await page.waitForTimeout(500);

        const outPath = path.join(IMG_BASE, theme.name, `${name}.png`);
        await page.screenshot({ path: outPath, fullPage: true });
        console.log(`    -> ${outPath}`);
      }

      await context.close();
    }

    await browser.close();
    console.log(`\nDone! Screenshots saved to docs/content/assets/img/{light,dark}/`);
  } finally {
    vite.kill("SIGTERM");
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
