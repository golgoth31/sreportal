import { describe, expect, it } from "vitest";

import type { MetricFamily } from "./metrics.types";
import {
  extractSubsystem,
  extractSubsystems,
  filterFamilies,
  formatMetricName,
} from "./metrics.types";

const gaugeFamily: MetricFamily = {
  name: "sreportal_dns_fqdns_total",
  help: "Total FQDNs per portal",
  type: "GAUGE",
  metrics: [
    { labels: { portal: "main", source: "external-dns" }, value: 5 },
    { labels: { portal: "main", source: "manual" }, value: 3 },
  ],
};

const counterFamily: MetricFamily = {
  name: "sreportal_controller_reconcile_total",
  help: "Total reconciliations",
  type: "COUNTER",
  metrics: [
    { labels: { controller: "dns", result: "success" }, value: 42 },
  ],
};

const histogramFamily: MetricFamily = {
  name: "sreportal_http_request_duration_seconds",
  help: "HTTP request latency",
  type: "HISTOGRAM",
  metrics: [
    {
      labels: { method: "GET", handler: "connect" },
      value: 0,
      histogram: {
        sampleCount: 100,
        sampleSum: 12.5,
        buckets: [
          { cumulativeCount: 50, upperBound: 0.01 },
          { cumulativeCount: 90, upperBound: 0.1 },
          { cumulativeCount: 100, upperBound: Infinity },
        ],
      },
    },
  ],
};

const allFamilies = [gaugeFamily, counterFamily, histogramFamily];

describe("extractSubsystem", () => {
  it("extracts subsystem from sreportal metric name", () => {
    expect(extractSubsystem("sreportal_dns_fqdns_total")).toBe("dns");
    expect(extractSubsystem("sreportal_controller_reconcile_total")).toBe(
      "controller",
    );
    expect(extractSubsystem("sreportal_http_request_duration_seconds")).toBe(
      "http",
    );
  });

  it("returns empty string for non-sreportal names", () => {
    expect(extractSubsystem("go_goroutines")).toBe("");
    expect(extractSubsystem("")).toBe("");
  });
});

describe("formatMetricName", () => {
  it("strips the sreportal_ prefix", () => {
    expect(formatMetricName("sreportal_dns_fqdns_total")).toBe(
      "dns_fqdns_total",
    );
  });

  it("returns name as-is if no prefix", () => {
    expect(formatMetricName("go_goroutines")).toBe("go_goroutines");
  });
});

describe("filterFamilies", () => {
  it("returns all families when no filters", () => {
    expect(filterFamilies(allFamilies, "", "")).toEqual(allFamilies);
  });

  it("filters by search substring on name", () => {
    const result = filterFamilies(allFamilies, "fqdn", "");
    expect(result).toHaveLength(1);
    expect(result[0]!.name).toBe("sreportal_dns_fqdns_total");
  });

  it("filters by subsystem", () => {
    const result = filterFamilies(allFamilies, "", "controller");
    expect(result).toHaveLength(1);
    expect(result[0]!.name).toBe("sreportal_controller_reconcile_total");
  });

  it("combines search and subsystem filters", () => {
    const result = filterFamilies(allFamilies, "reconcile", "controller");
    expect(result).toHaveLength(1);
    expect(result[0]!.name).toBe("sreportal_controller_reconcile_total");
  });

  it("returns empty array when nothing matches", () => {
    expect(filterFamilies(allFamilies, "nonexistent", "")).toEqual([]);
  });
});

describe("extractSubsystems", () => {
  it("extracts unique sorted subsystems", () => {
    expect(extractSubsystems(allFamilies)).toEqual([
      "controller",
      "dns",
      "http",
    ]);
  });

  it("returns empty array for empty input", () => {
    expect(extractSubsystems([])).toEqual([]);
  });
});
