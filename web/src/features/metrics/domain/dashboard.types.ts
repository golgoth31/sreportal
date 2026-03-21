import type { MetricFamily } from "./metrics.types";

/** Key stats extracted from well-known sreportal metrics for the dashboard. */
export interface DashboardStats {
  readonly totalFqdns: number;
  readonly activeAlerts: number;
  readonly totalPortals: number;
  readonly httpInFlight: number;
  readonly mcpSessions: number;
  readonly sourceEndpoints: number;
}

/** Data point for the FQDNs bar chart (by portal + source). */
export interface FqdnBarDataPoint {
  readonly label: string;
  readonly value: number;
  readonly portal: string;
  readonly source: string;
}

/** Data point for the portals donut chart (by type). */
export interface PortalDonutDataPoint {
  readonly type: string;
  readonly count: number;
  readonly fill: string;
}

const METRIC_NAMES = {
  fqdns: "sreportal_dns_fqdns_total",
  alerts: "sreportal_alertmanager_alerts_active",
  portals: "sreportal_portal_total",
  httpInFlight: "sreportal_http_requests_in_flight",
  mcpSessions: "sreportal_mcp_sessions_active",
  sourceEndpoints: "sreportal_source_endpoints_collected",
} as const;

const DONUT_COLORS: Record<string, string> = {
  local: "var(--chart-1)",
  remote: "var(--chart-2)",
};

function findFamily(
  families: readonly MetricFamily[],
  name: string,
): MetricFamily | undefined {
  return families.find((f) => f.name === name);
}

function sumMetricValues(family: MetricFamily | undefined): number {
  if (!family) return 0;
  return family.metrics.reduce((sum, m) => sum + m.value, 0);
}

/** Extracts key gauge values from MetricFamily[] for stat cards. */
export function extractDashboardStats(
  families: readonly MetricFamily[],
): DashboardStats {
  return {
    totalFqdns: sumMetricValues(findFamily(families, METRIC_NAMES.fqdns)),
    activeAlerts: sumMetricValues(findFamily(families, METRIC_NAMES.alerts)),
    totalPortals: sumMetricValues(findFamily(families, METRIC_NAMES.portals)),
    httpInFlight: sumMetricValues(
      findFamily(families, METRIC_NAMES.httpInFlight),
    ),
    mcpSessions: sumMetricValues(
      findFamily(families, METRIC_NAMES.mcpSessions),
    ),
    sourceEndpoints: sumMetricValues(
      findFamily(families, METRIC_NAMES.sourceEndpoints),
    ),
  };
}

/** Extracts bar chart data from sreportal_dns_fqdns_total (one bar per portal+source). */
export function extractFqdnBarData(
  families: readonly MetricFamily[],
): FqdnBarDataPoint[] {
  const family = findFamily(families, METRIC_NAMES.fqdns);
  if (!family) return [];

  return family.metrics.map((m) => ({
    label: `${m.labels["portal"] ?? "unknown"} / ${m.labels["source"] ?? "unknown"}`,
    value: m.value,
    portal: m.labels["portal"] ?? "unknown",
    source: m.labels["source"] ?? "unknown",
  }));
}

/** Extracts donut chart data from sreportal_portal_total (one slice per type). */
export function extractPortalDonutData(
  families: readonly MetricFamily[],
): PortalDonutDataPoint[] {
  const family = findFamily(families, METRIC_NAMES.portals);
  if (!family) return [];

  return family.metrics.map((m) => {
    const type = m.labels["type"] ?? "unknown";
    return {
      type,
      count: m.value,
      fill: DONUT_COLORS[type] ?? "var(--chart-3)",
    };
  });
}
