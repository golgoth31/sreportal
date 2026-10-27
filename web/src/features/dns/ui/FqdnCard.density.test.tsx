import { cleanup, render, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import type { Fqdn } from "../domain/dns.types";
import { FqdnCard } from "./FqdnCard";
import { FqdnGroupCard } from "./FqdnGroupCard";

const fqdn: Fqdn = {
  name: "api.example.com",
  source: "external-dns",
  groups: ["prod"],
  description: "API endpoint",
  recordType: "A",
  targets: ["10.0.0.1"],
  dnsResourceName: "api",
  dnsResourceNamespace: "default",
  syncStatus: "sync",
  portals: ["main"],
};

function renderWithTooltip(ui: ReactElement) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

afterEach(() => {
  cleanup();
});

describe("Fqdn dense Soft Material table", () => {
  it("renders aligned table columns for a group", () => {
    const { container } = renderWithTooltip(
      <FqdnGroupCard
        group={{ name: "prod", source: "external-dns", fqdns: [fqdn] }}
      />,
    );
    const table = within(container).getByRole("table");
    expect(within(table).getByRole("columnheader", { name: "FQDN" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Type" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Target" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Source" })).toBeInTheDocument();
    expect(within(table).getByRole("link", { name: /api\.example\.com/i }).className).toMatch(
      /font-mono/,
    );
    expect(within(table).getByText("10.0.0.1").className).toMatch(/truncate/);
  });

  it("puts source info on the source badge trigger", () => {
    const withOrigin: Fqdn = {
      ...fqdn,
      originRef: { kind: "Service", namespace: "default", name: "api" },
    };
    const { container } = renderWithTooltip(
      <FqdnGroupCard
        group={{ name: "prod", source: "external-dns", fqdns: [withOrigin] }}
      />,
    );
    const table = within(container).getByRole("table");
    const badge = within(table).getByText("External DNS");
    expect(badge.className).toMatch(/font-mono/);
    expect(badge).toHaveAttribute("data-slot", "tooltip-trigger");
  });

  it("renders a table row for a single FQDN when wrapped in a table body", () => {
    const { container } = renderWithTooltip(
      <table>
        <tbody>
          <FqdnCard fqdn={fqdn} />
        </tbody>
      </table>,
    );
    expect(container.querySelector("tr")).toBeTruthy();
    expect(container.querySelectorAll("td").length).toBeGreaterThanOrEqual(5);
  });
});
