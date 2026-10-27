import { cleanup, render, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import type { Fqdn } from "../domain/dns.types";
import { FqdnCard } from "./FqdnCard";
import { FqdnGroupList } from "./FqdnGroupList";

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

const fqdnB: Fqdn = {
  ...fqdn,
  name: "web.example.com",
  groups: ["edge"],
  targets: ["10.0.0.2"],
};

function renderWithTooltip(ui: ReactElement) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

afterEach(() => {
  cleanup();
});

describe("Fqdn dense Soft Material table", () => {
  it("uses one page-level headerless table across groups", () => {
    const { container } = renderWithTooltip(
      <FqdnGroupList
        groups={[
          { name: "prod", source: "external-dns", fqdns: [fqdn] },
          { name: "edge", source: "external-dns", fqdns: [fqdnB] },
        ]}
        isLoading={false}
        hasFilters={false}
        onClearFilters={() => {}}
      />,
    );
    expect(within(container).getAllByRole("table")).toHaveLength(1);
    const table = within(container).getByRole("table");
    expect(within(table).queryByRole("columnheader")).toBeNull();
    expect(table.className).toMatch(/table-fixed/);
    expect(within(table).getByRole("link", { name: /api\.example\.com/i }).className).toMatch(
      /font-mono/,
    );
    expect(within(table).getByRole("link", { name: /web\.example\.com/i })).toBeInTheDocument();
  });

  it("puts source info on the source badge trigger", () => {
    const withOrigin: Fqdn = {
      ...fqdn,
      originRef: { kind: "Service", namespace: "default", name: "api" },
    };
    const { container } = renderWithTooltip(
      <FqdnGroupList
        groups={[{ name: "prod", source: "external-dns", fqdns: [withOrigin] }]}
        isLoading={false}
        hasFilters={false}
        onClearFilters={() => {}}
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
