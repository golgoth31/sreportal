import { render, screen, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

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

describe("Fqdn dense Soft Material rows", () => {
  it("renders a dense mono row shell", () => {
    const { container } = renderWithTooltip(<FqdnCard fqdn={fqdn} />);
    const root = container.firstElementChild as HTMLElement;
    expect(root.className).toMatch(/min-h-\[var\(--row-height\)\]/);
    expect(root.className).toMatch(/flex-row/);
    expect(screen.getByRole("link", { name: /api\.example\.com/i }).className).toMatch(
      /font-mono/,
    );
    expect(screen.getByText("10.0.0.1").className).toMatch(/font-mono/);
  });

  it("lists FQDNs in a single column (no multi-col grid)", () => {
    const { container } = renderWithTooltip(
      <FqdnGroupCard
        group={{ name: "prod", source: "external-dns", fqdns: [fqdn] }}
      />,
    );
    const grid = container.querySelector(".border-t");
    expect(grid?.className).not.toMatch(/sm:grid-cols-2/);
    expect(grid?.className).not.toMatch(/lg:grid-cols-3/);
    expect(grid?.className).toMatch(/gap-\[var\(--density-gap\)\]/);
  });

  it("puts source info on the source badge trigger", () => {
    const withOrigin: Fqdn = {
      ...fqdn,
      originRef: { kind: "Service", namespace: "default", name: "api" },
    };
    const { container } = renderWithTooltip(<FqdnCard fqdn={withOrigin} />);
    const row = container.firstElementChild as HTMLElement;
    const badge = within(row).getByText("External DNS");
    expect(badge.className).toMatch(/font-mono/);
    // Source badge is the tooltip trigger (Radix may omit data-slot on asChild).
    expect(badge.getAttribute("data-state") ?? badge.parentElement?.getAttribute("data-state")).toBeTruthy();
  });
});
