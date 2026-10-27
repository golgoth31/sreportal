import { readFileSync } from "node:fs";
import path from "node:path";

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

  it("aligns --radius-md with --radius", () => {
    // @theme inline does not emit --radius-md on :root; assert the mapping in source.
    const css = readFileSync(path.join(process.cwd(), "src/index.css"), "utf8");
    const theme = css.slice(css.indexOf("@theme inline"));
    const radiusMd = theme.match(/--radius-md:\s*([^;]+);/)?.[1]?.trim();
    expect(radiusMd).toBe("var(--radius)");
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
