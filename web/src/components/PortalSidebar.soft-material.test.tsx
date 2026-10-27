import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MemoryRouter } from "react-router";

import { PortalSidebar } from "@/components/PortalSidebar";
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
  it("active nav indicator uses a 3px inset bar", () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/main/links"]}>
        <PortalSidebar portalName="main" portals={[portal]} />
      </MemoryRouter>,
    );
    const link = container.querySelector('a[href="/main/links"]');
    expect(link?.className).toMatch(/before:w-\[3px\]/);
    expect(link?.className).not.toMatch(/before:w-\[2px\]/);
  });
});
