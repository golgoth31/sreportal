import { render } from "@testing-library/react";
import { Bar, BarChart, XAxis } from "recharts";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChartContainer, type ChartConfig } from "./chart";

const minimalConfig = {
  value: {
    label: "Test",
    color: "var(--chart-1)",
  },
} satisfies ChartConfig;

function MinimalChart() {
  return (
    <ChartContainer config={minimalConfig}>
      <BarChart accessibilityLayer data={[{ label: "a", value: 1 }]}>
        <XAxis dataKey="label" />
        <Bar dataKey="value" fill="var(--color-value)" radius={4} />
      </BarChart>
    </ChartContainer>
  );
}

describe("ChartContainer", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does not emit Recharts ResponsiveContainer size warning on mount", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(<MinimalChart />);

    const rechartsSizeWarning = warn.mock.calls.some(
      (args) =>
        typeof args[0] === "string" &&
        args[0].includes("of chart should be greater than 0"),
    );

    expect(rechartsSizeWarning).toBe(false);
  });
});
