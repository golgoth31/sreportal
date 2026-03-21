import { render, screen } from "@testing-library/react";
import { Globe } from "lucide-react";
import { describe, expect, it } from "vitest";

import { StatCard } from "./StatCard";

describe("StatCard", () => {
  it("displays title and formatted value", () => {
    render(<StatCard title="Total FQDNs" value={1234} icon={Globe} />);

    expect(screen.getByText("Total FQDNs")).toBeInTheDocument();
    expect(screen.getByText("1,234")).toBeInTheDocument();
  });

  it("renders accessible value label", () => {
    render(<StatCard title="Active Alerts" value={5} icon={Globe} />);

    expect(
      screen.getByLabelText("Active Alerts: 5"),
    ).toBeInTheDocument();
  });

  it("renders description when provided", () => {
    render(
      <StatCard
        title="Portals"
        value={3}
        icon={Globe}
        description="2 local, 1 remote"
      />,
    );

    expect(screen.getByText("2 local, 1 remote")).toBeInTheDocument();
  });

  it("does not render description when omitted", () => {
    const { container } = render(
      <StatCard title="HTTP In-Flight" value={0} icon={Globe} />,
    );

    expect(
      container.querySelectorAll(".text-xs.text-muted-foreground"),
    ).toHaveLength(0);
  });
});
