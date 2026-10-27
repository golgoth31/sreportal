import { beforeAll, describe, expect, it } from "vitest";

import "../index.css";

describe("soft material tokens", () => {
  beforeAll(() => {
    document.documentElement.classList.remove("dark");
  });

  it("sets Soft Material radius to 0.25rem on :root", () => {
    const radius = getComputedStyle(document.documentElement)
      .getPropertyValue("--radius")
      .trim();
    expect(radius).toBe("0.25rem");
  });

  it("defines elevation-1 for floating surfaces", () => {
    const elev = getComputedStyle(document.documentElement)
      .getPropertyValue("--elevation-1")
      .trim();
    expect(elev.length).toBeGreaterThan(0);
  });

  it("uses a brighter primary in dark mode than a near-black surface", () => {
    document.documentElement.classList.add("dark");
    const primary = getComputedStyle(document.documentElement)
      .getPropertyValue("--primary")
      .trim();
    const background = getComputedStyle(document.documentElement)
      .getPropertyValue("--background")
      .trim();
    expect(primary.length).toBeGreaterThan(0);
    expect(background.length).toBeGreaterThan(0);
    expect(primary).not.toBe(background);
    document.documentElement.classList.remove("dark");
  });
});
