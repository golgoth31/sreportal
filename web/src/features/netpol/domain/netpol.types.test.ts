import { describe, it, expect } from "vitest";

import type { NetpolNode, NetpolEdge } from "./netpol.types";
import { buildFlowMaps, computeImpact } from "./netpol.types";

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

describe("computeImpact", () => {
  it("returns only the target when it has no dependents", () => {
    const nodes = new Map([["a", node({ id: "a" })]]);
    const callsFrom = new Map<string, NetpolEdge[]>();

    const levels = computeImpact("a", nodes, callsFrom);
    expect(levels).toHaveLength(1);
    expect(levels[0]!.depth).toBe(0);
    expect(levels[0]!.nodes[0]!.node.id).toBe("a");
  });

  it("returns empty level-0 nodes for unknown target", () => {
    const nodes = new Map<string, NetpolNode>();
    const callsFrom = new Map<string, NetpolEdge[]>();

    const levels = computeImpact("unknown", nodes, callsFrom);
    expect(levels).toHaveLength(1);
    expect(levels[0]!.nodes).toHaveLength(0);
  });

  it("computes direct dependents at depth 1", () => {
    const nodes = new Map([
      ["db", node({ id: "db", nodeType: "database" })],
      ["svc-a", node({ id: "svc-a" })],
      ["svc-b", node({ id: "svc-b" })],
    ]);
    // svc-a -> db, svc-b -> db (both call db)
    const callsFrom = new Map([
      ["db", [edge("svc-a", "db"), edge("svc-b", "db")]],
    ]);

    const levels = computeImpact("db", nodes, callsFrom);
    expect(levels).toHaveLength(2);
    expect(levels[1]!.depth).toBe(1);
    expect(levels[1]!.nodes).toHaveLength(2);
  });

  it("computes transitive dependents across multiple levels", () => {
    // chain: d -> c -> b -> a (a is the target)
    const nodes = new Map([
      ["a", node({ id: "a" })],
      ["b", node({ id: "b" })],
      ["c", node({ id: "c" })],
      ["d", node({ id: "d" })],
    ]);
    const callsFrom = new Map([
      ["a", [edge("b", "a")]],
      ["b", [edge("c", "b")]],
      ["c", [edge("d", "c")]],
    ]);

    const levels = computeImpact("a", nodes, callsFrom);
    expect(levels).toHaveLength(4); // depth 0,1,2,3
    expect(levels[1]!.nodes[0]!.node.id).toBe("b");
    expect(levels[2]!.nodes[0]!.node.id).toBe("c");
    expect(levels[3]!.nodes[0]!.node.id).toBe("d");
  });

  it("does not visit the same node twice (cycle protection)", () => {
    // a -> b -> a (cycle)
    const nodes = new Map([
      ["a", node({ id: "a" })],
      ["b", node({ id: "b" })],
    ]);
    const callsFrom = new Map([
      ["a", [edge("b", "a")]],
      ["b", [edge("a", "b")]],
    ]);

    const levels = computeImpact("a", nodes, callsFrom);
    expect(levels).toHaveLength(2); // depth 0 (a) + depth 1 (b), then stops
  });

  it("respects maxDepth parameter", () => {
    const nodes = new Map([
      ["a", node({ id: "a" })],
      ["b", node({ id: "b" })],
      ["c", node({ id: "c" })],
    ]);
    const callsFrom = new Map([
      ["a", [edge("b", "a")]],
      ["b", [edge("c", "b")]],
    ]);

    const levels = computeImpact("a", nodes, callsFrom, 1);
    expect(levels).toHaveLength(2); // depth 0 + depth 1, stops before depth 2
  });
});
