import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { RootLayout } from "@/components/RootLayout";

vi.mock("@/features/portal/hooks/usePortals", () => ({
  usePortals: () => ({ portals: [], isLoading: false }),
}));

vi.mock("@/features/version/hooks/useVersion", () => ({
  useVersion: () => ({ version: "test" }),
}));

describe("RootLayout soft material shell", () => {
  it("app bar is opaque without backdrop blur", () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<RootLayout />}>
            <Route index element={<div>home</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    const header = container.querySelector("header");
    expect(header?.className).not.toMatch(/backdrop-blur/);
    expect(header?.className).toMatch(/bg-background/);
    expect(header?.className).toMatch(/shadow-\[var\(--elevation-2\)\]/);
    const nav = header?.querySelector('nav[aria-label="Portal navigation"]');
    expect(nav?.className).toMatch(/flex-nowrap/);
    expect(nav?.parentElement?.className).toMatch(/overflow-y-hidden/);
    expect(screen.getAllByText(/SRE/i).length).toBeGreaterThan(0);
  });
});
