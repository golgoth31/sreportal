import { afterEach, beforeAll, describe, expect, it } from "vitest";

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

function oklchHue(value: string): number {
  const match = /^oklch\(\s*[\d.]+\s+[\d.]+\s+([\d.]+)/.exec(value);
  if (!match) throw new Error(`not an oklch color with hue: "${value}"`);
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

  it("separates dark card surfaces from the background", () => {
    document.documentElement.classList.add("dark");
    const card = oklchLightness(token("--card"));
    const background = oklchLightness(token("--background"));
    expect(card - background).toBeGreaterThanOrEqual(0.05);
  });

  it("keeps dark popover and accent on indigo hue 260", () => {
    document.documentElement.classList.add("dark");
    expect(oklchHue(token("--popover"))).toBe(260);
    expect(oklchHue(token("--popover-foreground"))).toBe(260);
    expect(oklchHue(token("--accent"))).toBe(260);
    expect(oklchHue(token("--accent-foreground"))).toBe(260);
  });
});
