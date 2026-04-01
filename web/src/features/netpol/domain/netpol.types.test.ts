import { describe, it, expect } from "vitest";

import type { NetpolNode, NetpolEdge } from "./netpol.types";
import { buildFlowMaps } from "./netpol.types";

function node(overrides: Partial<NetpolNode> & { id: string }): NetpolNode {
  return {
    label: overrides.id,
    namespace: "default",
    nodeType: "service",
    group: "default",
    ...overrides,
  };
}

function edge(from: string, to: string, edgeType = "allow"): NetpolEdge {
  return { from, to, edgeType };
}

describe("buildFlowMaps", () => {
  it("returns empty maps for no edges", () => {
    const { callsTo, callsFrom } = buildFlowMaps([]);
    expect(callsTo.size).toBe(0);
    expect(callsFrom.size).toBe(0);
  });

  it("builds callsTo lookup (outgoing edges by source)", () => {
    const edges = [edge("a", "b"), edge("a", "c")];
    const { callsTo } = buildFlowMaps(edges);
    expect(callsTo.get("a")).toHaveLength(2);
    expect(callsTo.get("b")).toBeUndefined();
  });

  it("builds callsFrom lookup (incoming edges by target)", () => {
    const edges = [edge("a", "b"), edge("c", "b")];
    const { callsFrom } = buildFlowMaps(edges);
    expect(callsFrom.get("b")).toHaveLength(2);
    expect(callsFrom.get("a")).toBeUndefined();
  });

  it("handles a node that is both source and target", () => {
    const edges = [edge("a", "b"), edge("b", "c")];
    const { callsTo, callsFrom } = buildFlowMaps(edges);
    expect(callsTo.get("b")).toHaveLength(1);
    expect(callsFrom.get("b")).toHaveLength(1);
  });
});

