import { afterEach, beforeAll, describe, expect, it } from "vitest";

import css from "../index.css?raw";
import "../index.css";

function token(name: string): string {
  return getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
}

function oklchLightness(value: string): number {
  const match = /^oklch\(\s*([\d.]+)/.exec(value);
  if (!match) throw new Error(`not an oklch color: "${value}"`);
  return Number(match[1]);
}

describe("soft material tokens", () => {
  beforeAll(() => {
    document.documentElement.classList.remove("dark");
  });

  afterEach(() => {
    document.documentElement.classList.remove("dark");
  });

  it("sets Soft Material radius to 0.25rem on :root", () => {
    expect(token("--radius")).toBe("0.25rem");
  });

  it("aligns --radius-md with --radius", () => {
    // @theme inline does not emit --radius-md on :root; assert the mapping in source.
    expect(css).toMatch(/^\s*--radius-md:\s*var\(--radius\);/m);
  });

  it("defines elevation-1 and elevation-2 in light mode", () => {
    expect(token("--elevation-1")).toContain("rgb(15 23 42");
    expect(token("--elevation-2")).toContain("rgb(15 23 42");
  });

  it("defines darker elevation-1 and elevation-2 in dark mode", () => {
    document.documentElement.classList.add("dark");
    expect(token("--elevation-1")).toContain("rgb(0 0 0");
    expect(token("--elevation-2")).toContain("rgb(0 0 0");
  });

  it("uses a brighter primary in dark mode than a near-black surface", () => {
    document.documentElement.classList.add("dark");
    const primary = oklchLightness(token("--primary"));
    const background = oklchLightness(token("--background"));
    expect(primary).toBeGreaterThan(0.6);
    expect(background).toBeLessThan(0.25);
  });
});
