import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { MetricFamily } from "../domain/metrics.types";
import { MetricFamilyCard } from "./MetricFamilyCard";

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
      labels: { method: "GET" },
      value: 0,
      histogram: {
        sampleCount: 100,
        sampleSum: 12.5,
        buckets: [
          { cumulativeCount: 90, upperBound: 0.1 },
          { cumulativeCount: 100, upperBound: Infinity },
        ],
      },
    },
  ],
};

describe("MetricFamilyCard", () => {
  it("displays formatted name and type badge", () => {
    render(<MetricFamilyCard family={gaugeFamily} />);

    expect(screen.getByText("dns_fqdns_total")).toBeInTheDocument();
    expect(screen.getByText("GAUGE")).toBeInTheDocument();
    expect(screen.getByText("2 series")).toBeInTheDocument();
  });

  it("shows metric values when expanded", async () => {
    const user = userEvent.setup();
    render(<MetricFamilyCard family={counterFamily} />);

    await user.click(screen.getByText("controller_reconcile_total"));

    expect(screen.getByText("Total reconciliations")).toBeInTheDocument();
    expect(screen.getByText("dns")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("shows histogram count and sum when expanded", async () => {
    const user = userEvent.setup();
    render(<MetricFamilyCard family={histogramFamily} />);

    await user.click(screen.getByText("http_request_duration_seconds"));

    expect(screen.getByText("Count")).toBeInTheDocument();
    expect(screen.getByText("Sum")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByText("12.50")).toBeInTheDocument();
  });

  it("shows label column headers when expanded", async () => {
    const user = userEvent.setup();
    render(<MetricFamilyCard family={gaugeFamily} />);

    await user.click(screen.getAllByText("dns_fqdns_total")[0]!);

    expect(screen.getByText("portal")).toBeInTheDocument();
    expect(screen.getByText("source")).toBeInTheDocument();
  });
});
