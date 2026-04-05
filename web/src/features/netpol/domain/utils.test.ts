import { describe, it, expect } from "vitest";

import { groupColor, dedup, formatLastSeen, GROUP_PALETTE } from "./utils";

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

describe("formatLastSeen", () => {
  it("returns 'never' for null", () => {
    expect(formatLastSeen(null)).toBe("never");
  });

  it("returns 'just now' for timestamps less than 1 minute ago", () => {
    const now = new Date().toISOString();
    expect(formatLastSeen(now)).toBe("just now");
  });

  it("returns minutes ago for timestamps less than 1 hour ago", () => {
    const fiveMinAgo = new Date(Date.now() - 5 * 60_000).toISOString();
    expect(formatLastSeen(fiveMinAgo)).toBe("5m ago");
  });

  it("returns hours ago for timestamps less than 1 day ago", () => {
    const twoHoursAgo = new Date(Date.now() - 2 * 3_600_000).toISOString();
    expect(formatLastSeen(twoHoursAgo)).toBe("2h ago");
  });

  it("returns days ago for older timestamps", () => {
    const threeDaysAgo = new Date(Date.now() - 3 * 86_400_000).toISOString();
    expect(formatLastSeen(threeDaysAgo)).toBe("3d ago");
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
