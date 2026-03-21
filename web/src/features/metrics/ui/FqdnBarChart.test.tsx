import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { FqdnBarDataPoint } from "../domain/dashboard.types";

import { FqdnBarChart } from "./FqdnBarChart";

const data: FqdnBarDataPoint[] = [
  { label: "main / external-dns", value: 5, portal: "main", source: "external-dns" },
  { label: "main / manual", value: 3, portal: "main", source: "manual" },
];

describe("FqdnBarChart", () => {
  it("renders the card title", () => {
    render(<FqdnBarChart data={data} />);

    expect(screen.getByText("FQDNs by Portal")).toBeInTheDocument();
    expect(
      screen.getByText("Distribution by portal and source"),
    ).toBeInTheDocument();
  });

  it("renders empty state when data is empty", () => {
    render(<FqdnBarChart data={[]} />);

    expect(screen.getByText("No data available")).toBeInTheDocument();
  });
});
