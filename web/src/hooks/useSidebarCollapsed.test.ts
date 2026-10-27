import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { useSidebarCollapsed } from "./useSidebarCollapsed";

describe("useSidebarCollapsed", () => {
  afterEach(() => {
    localStorage.removeItem("sreportal-sidebar-collapsed");
  });

  it("defaults to expanded", () => {
    const { result } = renderHook(() => useSidebarCollapsed());
    expect(result.current.collapsed).toBe(false);
  });

  it("persists toggle to localStorage", () => {
    const { result } = renderHook(() => useSidebarCollapsed());
    act(() => {
      result.current.toggle();
    });
    expect(result.current.collapsed).toBe(true);
    expect(localStorage.getItem("sreportal-sidebar-collapsed")).toBe("true");
  });

  it("reads stored collapsed=true on init", () => {
    localStorage.setItem("sreportal-sidebar-collapsed", "true");
    const { result } = renderHook(() => useSidebarCollapsed());
    expect(result.current.collapsed).toBe(true);
  });
});
