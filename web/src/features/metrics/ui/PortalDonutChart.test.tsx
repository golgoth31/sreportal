import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { PortalDonutDataPoint } from "../domain/dashboard.types";

import { PortalDonutChart } from "./PortalDonutChart";

const data: PortalDonutDataPoint[] = [
  { type: "local", count: 2, fill: "var(--chart-1)" },
  { type: "remote", count: 1, fill: "var(--chart-2)" },
];

describe("PortalDonutChart", () => {
  it("renders the card title", () => {
    render(<PortalDonutChart data={data} />);

    expect(screen.getByText("Portals by Type")).toBeInTheDocument();
    expect(
      screen.getByText("Local vs remote distribution"),
    ).toBeInTheDocument();
  });

  it("renders empty state when data is empty", () => {
    render(<PortalDonutChart data={[]} />);

    expect(screen.getByText("No data available")).toBeInTheDocument();
  });
});
