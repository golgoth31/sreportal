import { describe, it, expect } from "vitest";

import { groupColor, dedup, GROUP_PALETTE } from "./utils";

describe("groupColor", () => {
  it("returns a string from GROUP_PALETTE", () => {
    const result = groupColor("my-namespace");
    expect(GROUP_PALETTE).toContain(result);
  });

  it("returns the same color for the same input", () => {
    expect(groupColor("production")).toBe(groupColor("production"));
  });

  it("returns different colors for different inputs (non-guaranteed but likely)", () => {
    const colors = new Set(
      ["ns-a", "ns-b", "ns-c", "ns-d", "ns-e"].map(groupColor)
    );
    expect(colors.size).toBeGreaterThan(1);
  });

  it("handles empty string without throwing", () => {
    expect(() => groupColor("")).not.toThrow();
    expect(GROUP_PALETTE).toContain(groupColor(""));
  });
});

describe("dedup", () => {
  it("returns empty array for no edges", () => {
    expect(dedup([])).toEqual([]);
  });

  it("removes duplicate edges by from|to|edgeType", () => {
    const edges = [
      { from: "a", to: "b", edgeType: "allow" },
      { from: "a", to: "b", edgeType: "allow" },
      { from: "a", to: "b", edgeType: "deny" },
    ];
    const result = dedup(edges);
    expect(result).toHaveLength(2);
  });

  it("keeps edges with different from/to pairs", () => {
    const edges = [
      { from: "a", to: "b", edgeType: "allow" },
      { from: "b", to: "a", edgeType: "allow" },
    ];
    expect(dedup(edges)).toHaveLength(2);
  });
});
