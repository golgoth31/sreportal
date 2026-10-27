import { cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { MemoryRouter } from "react-router";

import { PortalSidebar } from "@/components/PortalSidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Portal } from "@/features/portal/domain/portal.types";

const portal: Portal = {
  name: "main",
  title: "Main",
  main: true,
  subPath: "",
  namespace: "default",
  ready: true,
  url: "",
  isRemote: false,
  features: {
    dns: true,
    releases: false,
    networkPolicy: false,
    alerts: true,
    statusPage: false,
    imageInventory: false,
  },
};

describe("PortalSidebar soft material", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("active nav indicator uses a 3px inset bar", () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <TooltipProvider>
        <PortalSidebar portalName="main" portals={[portal]} />
        </TooltipProvider>
      </MemoryRouter>,
    );
    const link = container.querySelector('a[href="/main/links"]');
    expect(link?.className).toMatch(/before:w-\[3px\]/);
    expect(link?.className).not.toMatch(/before:w-\[2px\]/);
  });

  it("uses expanded sidebar width token by default", () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <TooltipProvider>
        <PortalSidebar portalName="main" portals={[portal]} />
        </TooltipProvider>
      </MemoryRouter>,
    );
    const aside = container.querySelector("aside");
    expect(aside?.className).toMatch(/w-\[var\(--sidebar-width\)\]/);
  });

  it("collapses to icon-only width when storage says collapsed", () => {
    localStorage.setItem("sreportal-sidebar-collapsed", "true");
    const { container } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <TooltipProvider>
        <PortalSidebar portalName="main" portals={[portal]} />
        </TooltipProvider>
      </MemoryRouter>,
    );
    const aside = container.querySelector("aside");
    expect(aside?.className).toMatch(/w-\[var\(--sidebar-width-collapsed\)\]/);
  });

  it("exposes an aria-expanded toggle", () => {
    const { getByRole } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <TooltipProvider>
        <PortalSidebar portalName="main" portals={[portal]} />
        </TooltipProvider>
      </MemoryRouter>,
    );
    expect(getByRole("button", { name: /collapse sidebar|expand sidebar/i })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
  });

  it("toggles collapsed state and updates label", () => {
    const { getByRole } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <TooltipProvider>
        <PortalSidebar portalName="main" portals={[portal]} />
        </TooltipProvider>
      </MemoryRouter>,
    );
    fireEvent.click(getByRole("button", { name: "Collapse sidebar" }));
    expect(getByRole("button", { name: "Expand sidebar" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
  });
});
