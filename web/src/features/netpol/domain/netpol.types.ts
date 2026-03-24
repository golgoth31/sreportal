/**
 * Domain types for Network Policy flow analysis.
 * No React or infrastructure dependencies.
 */

export interface NetpolNode {
  readonly id: string;
  readonly label: string;
  readonly namespace: string;
  readonly nodeType: string;
  readonly group: string;
}

export interface NetpolEdge {
  readonly from: string;
  readonly to: string;
  readonly edgeType: string;
}

export interface NetpolGraph {
  readonly nodes: readonly NetpolNode[];
  readonly edges: readonly NetpolEdge[];
}

export interface ServiceFlows {
  readonly service: NetpolNode;
  readonly callsTo: readonly FlowEntry[];
  readonly calledFrom: readonly FlowEntry[];
}

export interface FlowEntry {
  readonly node: NetpolNode;
  readonly edgeType: string;
}

export interface ImpactLevel {
  readonly depth: number;
  readonly nodes: readonly ImpactNode[];
}

export interface ImpactNode {
  readonly node: NetpolNode;
  readonly via: string;
}

/** Build callsTo/callsFrom lookup maps from edges */
export function buildFlowMaps(edges: readonly NetpolEdge[]) {
  const callsTo = new Map<string, NetpolEdge[]>();
  const callsFrom = new Map<string, NetpolEdge[]>();

  for (const e of edges) {
    const to = callsTo.get(e.from) ?? [];
    to.push(e);
    callsTo.set(e.from, to);

    const from = callsFrom.get(e.to) ?? [];
    from.push(e);
    callsFrom.set(e.to, from);
  }

  return { callsTo, callsFrom };
}

/** BFS impact analysis: who is affected if a node goes down */
export function computeImpact(
  targetId: string,
  nodes: ReadonlyMap<string, NetpolNode>,
  callsFrom: ReadonlyMap<string, readonly NetpolEdge[]>,
  maxDepth = 6
): ImpactLevel[] {
  const visited = new Set([targetId]);
  let currentLevel = new Set([targetId]);
  const target = nodes.get(targetId);

  const levels: ImpactLevel[] = [
    { depth: 0, nodes: target ? [{ node: target, via: "" }] : [] },
  ];

  for (let depth = 1; depth <= maxDepth; depth++) {
    const nextNodes: ImpactNode[] = [];
    const nextLevel = new Set<string>();

    for (const nid of currentLevel) {
      for (const e of callsFrom.get(nid) ?? []) {
        if (!visited.has(e.from)) {
          visited.add(e.from);
          nextLevel.add(e.from);
          const via = nodes.get(nid)?.label ?? "";
          const node = nodes.get(e.from);
          if (node) nextNodes.push({ node, via });
        }
      }
    }

    if (nextNodes.length === 0) break;
    nextNodes.sort((a, b) => a.node.label.localeCompare(b.node.label));
    levels.push({ depth, nodes: nextNodes });
    currentLevel = nextLevel;
  }

  return levels;
}
