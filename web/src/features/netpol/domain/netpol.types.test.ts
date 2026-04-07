import { describe, expect, it } from "vitest";

import type { EdgeType, NetpolEdge } from "./netpol.types";
import { buildFlowMaps } from "./netpol.types";

function mkEdge(from: string, to: string, edgeType: EdgeType = "internal"): NetpolEdge {
  return { from, to, edgeType, used: false };
}

describe("buildFlowMaps", () => {
  it("returns empty maps for no edges", () => {
    const { callsTo, callsFrom } = buildFlowMaps([]);
    expect(callsTo.size).toBe(0);
    expect(callsFrom.size).toBe(0);
  });

  it("builds callsTo lookup (outgoing edges by source)", () => {
    const edges = [mkEdge("a", "b"), mkEdge("a", "c")];
    const { callsTo } = buildFlowMaps(edges);
    expect(callsTo.get("a")).toHaveLength(2);
    expect(callsTo.get("b")).toBeUndefined();
  });

  it("builds callsFrom lookup (incoming edges by target)", () => {
    const edges = [mkEdge("a", "b"), mkEdge("c", "b")];
    const { callsFrom } = buildFlowMaps(edges);
    expect(callsFrom.get("b")).toHaveLength(2);
    expect(callsFrom.get("a")).toBeUndefined();
  });

  it("handles a node that is both source and target", () => {
    const edges = [mkEdge("a", "b"), mkEdge("b", "c")];
    const { callsTo, callsFrom } = buildFlowMaps(edges);
    expect(callsTo.get("b")).toHaveLength(1);
    expect(callsFrom.get("b")).toHaveLength(1);
  });
});

