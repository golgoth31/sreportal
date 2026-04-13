/**
 * Domain types for Network Policy flow analysis.
 * No React or infrastructure dependencies.
 */

export type NodeType = "service" | "database" | "cron" | "messaging" | "external";

export type EdgeType = "internal" | "cross-ns" | "cron" | "database" | "messaging" | "external";

export interface NetpolNode {
  readonly id: string;
  readonly label: string;
  readonly namespace: string;
  readonly nodeType: NodeType;
  readonly group: string;
}

export interface NetpolEdge {
  readonly from: string;
  readonly to: string;
  readonly edgeType: EdgeType;
  readonly used: boolean;
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

