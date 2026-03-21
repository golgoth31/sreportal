import { describe, expect, it } from "vitest";

import type { MetricFamily } from "./metrics.types";
import {
  extractDashboardStats,
  extractFqdnBarData,
  extractPortalDonutData,
} from "./dashboard.types";

const fqdnFamily: MetricFamily = {
  name: "sreportal_dns_fqdns_total",
  help: "Total FQDNs",
  type: "GAUGE",
  metrics: [
    { labels: { portal: "main", source: "external-dns" }, value: 5 },
    { labels: { portal: "main", source: "manual" }, value: 3 },
    { labels: { portal: "staging", source: "external-dns" }, value: 2 },
  ],
};

const alertsFamily: MetricFamily = {
  name: "sreportal_alertmanager_alerts_active",
  help: "Active alerts",
  type: "GAUGE",
  metrics: [
    { labels: { portal: "main", alertmanager: "prom" }, value: 4 },
    { labels: { portal: "staging", alertmanager: "prom-stg" }, value: 1 },
  ],
};

const portalsFamily: MetricFamily = {
  name: "sreportal_portal_total",
  help: "Portals by type",
  type: "GAUGE",
  metrics: [
    { labels: { type: "local" }, value: 2 },
    { labels: { type: "remote" }, value: 1 },
  ],
};

const httpInFlightFamily: MetricFamily = {
  name: "sreportal_http_requests_in_flight",
  help: "In-flight HTTP requests",
  type: "GAUGE",
  metrics: [{ labels: {}, value: 7 }],
};

const mcpSessionsFamily: MetricFamily = {
  name: "sreportal_mcp_sessions_active",
  help: "Active MCP sessions",
  type: "GAUGE",
  metrics: [
    { labels: { server: "dns" }, value: 2 },
    { labels: { server: "alerts" }, value: 1 },
  ],
};

const sourceEndpointsFamily: MetricFamily = {
  name: "sreportal_source_endpoints_collected",
  help: "Endpoints collected",
  type: "GAUGE",
  metrics: [
    { labels: { source_type: "service" }, value: 10 },
    { labels: { source_type: "ingress" }, value: 6 },
  ],
};

const allFamilies: MetricFamily[] = [
  fqdnFamily,
  alertsFamily,
  portalsFamily,
  httpInFlightFamily,
  mcpSessionsFamily,
  sourceEndpointsFamily,
];

describe("extractDashboardStats", () => {
  it("extracts all stat values by summing metrics", () => {
    const stats = extractDashboardStats(allFamilies);

    expect(stats.totalFqdns).toBe(10);
    expect(stats.activeAlerts).toBe(5);
    expect(stats.totalPortals).toBe(3);
    expect(stats.httpInFlight).toBe(7);
    expect(stats.mcpSessions).toBe(3);
    expect(stats.sourceEndpoints).toBe(16);
  });

  it("returns zeros when families are empty", () => {
    const stats = extractDashboardStats([]);

    expect(stats.totalFqdns).toBe(0);
    expect(stats.activeAlerts).toBe(0);
    expect(stats.totalPortals).toBe(0);
    expect(stats.httpInFlight).toBe(0);
    expect(stats.mcpSessions).toBe(0);
    expect(stats.sourceEndpoints).toBe(0);
  });

  it("returns zero for missing families", () => {
    const stats = extractDashboardStats([fqdnFamily]);

    expect(stats.totalFqdns).toBe(10);
    expect(stats.activeAlerts).toBe(0);
    expect(stats.totalPortals).toBe(0);
  });
});

describe("extractFqdnBarData", () => {
  it("returns one data point per portal+source combination", () => {
    const data = extractFqdnBarData(allFamilies);

    expect(data).toHaveLength(3);
    expect(data[0]).toEqual({
      label: "main / external-dns",
      value: 5,
      portal: "main",
      source: "external-dns",
    });
    expect(data[1]).toEqual({
      label: "main / manual",
      value: 3,
      portal: "main",
      source: "manual",
    });
    expect(data[2]).toEqual({
      label: "staging / external-dns",
      value: 2,
      portal: "staging",
      source: "external-dns",
    });
  });

  it("returns empty array when fqdns family is missing", () => {
    expect(extractFqdnBarData([portalsFamily])).toEqual([]);
  });

  it("returns empty array for empty input", () => {
    expect(extractFqdnBarData([])).toEqual([]);
  });
});

describe("extractPortalDonutData", () => {
  it("returns one data point per portal type with fill colors", () => {
    const data = extractPortalDonutData(allFamilies);

    expect(data).toHaveLength(2);
    expect(data[0]).toEqual({
      type: "local",
      count: 2,
      fill: "var(--chart-1)",
    });
    expect(data[1]).toEqual({
      type: "remote",
      count: 1,
      fill: "var(--chart-2)",
    });
  });

  it("uses fallback color for unknown type", () => {
    const families: MetricFamily[] = [
      {
        name: "sreportal_portal_total",
        help: "",
        type: "GAUGE",
        metrics: [{ labels: { type: "federated" }, value: 1 }],
      },
    ];
    const data = extractPortalDonutData(families);

    expect(data[0]!.fill).toBe("var(--chart-3)");
  });

  it("returns empty array when portals family is missing", () => {
    expect(extractPortalDonutData([fqdnFamily])).toEqual([]);
  });
});
