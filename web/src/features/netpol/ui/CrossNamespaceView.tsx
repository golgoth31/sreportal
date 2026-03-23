import { useMemo } from "react";

import { Badge } from "@/components/ui/badge";
import type { NetpolNode, NetpolEdge } from "../domain/netpol.types";

const GROUP_PALETTE = [
  "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  "bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200",
  "bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200",
  "bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200",
  "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  "bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200",
];

function groupColor(group: string): string {
  let hash = 0;
  for (let i = 0; i < group.length; i++) hash = (hash * 31 + group.charCodeAt(i)) | 0;
  return GROUP_PALETTE[Math.abs(hash) % GROUP_PALETTE.length]!;
}

interface Props {
  nodes: readonly NetpolNode[];
  edges: readonly NetpolEdge[];
  nodeMap: ReadonlyMap<string, NetpolNode>;
}

interface CrossPLFlow {
  sourcePl: string;
  targetPl: string;
  count: number;
  details: { source: string; target: string }[];
}

export function CrossNamespaceView({ edges, nodeMap }: Props) {
  const flows = useMemo(() => {
    const map = new Map<string, CrossPLFlow>();

    for (const e of edges) {
      if (e.edgeType !== "cross-ns") continue;
      const src = nodeMap.get(e.from);
      const tgt = nodeMap.get(e.to);
      if (!src || !tgt || src.group === tgt.group) continue;

      const key = `${src.group}→${tgt.group}`;
      const existing = map.get(key);
      if (existing) {
        existing.count++;
        existing.details.push({ source: src.label, target: tgt.label });
      } else {
        map.set(key, {
          sourcePl: src.group,
          targetPl: tgt.group,
          count: 1,
          details: [{ source: src.label, target: tgt.label }],
        });
      }
    }

    return [...map.values()].sort((a, b) => b.count - a.count);
  }, [edges, nodeMap]);

  const allPls = useMemo(
    () => [...new Set(flows.flatMap((f) => [f.sourcePl, f.targetPl]))].sort(),
    [flows]
  );

  // Build matrix
  const matrix = useMemo(() => {
    const m = new Map<string, Map<string, number>>();
    for (const pl of allPls) {
      m.set(pl, new Map());
    }
    for (const f of flows) {
      m.get(f.sourcePl)?.set(f.targetPl, f.count);
    }
    return m;
  }, [flows, allPls]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">Cross-Namespace Flows</h2>
        <p className="text-sm text-muted-foreground">
          {flows.length} cross-namespace flow pairs, {flows.reduce((s, f) => s + f.count, 0)} total service-to-service flows
        </p>
      </div>

      {/* Matrix table */}
      <div className="overflow-x-auto">
        <table className="text-sm border-collapse">
          <thead>
            <tr>
              <th className="p-2 text-left text-muted-foreground text-xs">From ↓ / To →</th>
              {allPls.map((pl) => (
                <th key={pl} className="p-2 text-center">
                  <Badge variant="outline" className={groupColor(pl)}>{pl}</Badge>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {allPls.map((srcPl) => (
              <tr key={srcPl} className="border-t border-border">
                <td className="p-2">
                  <Badge variant="outline" className={groupColor(srcPl)}>{srcPl}</Badge>
                </td>
                {allPls.map((tgtPl) => {
                  const count = matrix.get(srcPl)?.get(tgtPl) ?? 0;
                  return (
                    <td key={tgtPl} className="p-2 text-center">
                      {srcPl === tgtPl ? (
                        <span className="text-muted-foreground">—</span>
                      ) : count > 0 ? (
                        <span className="font-medium">{count}</span>
                      ) : (
                        <span className="text-muted-foreground/30">0</span>
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Top flows */}
      <div className="space-y-2">
        <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wide">Top flows</h3>
        {flows.slice(0, 20).map((f, i) => (
          <div key={i} className="flex items-center gap-2 text-sm">
            <Badge variant="outline" className={groupColor(f.sourcePl)}>{f.sourcePl}</Badge>
            <span className="text-muted-foreground">→</span>
            <Badge variant="outline" className={groupColor(f.targetPl)}>{f.targetPl}</Badge>
            <span className="font-medium">{f.count} flows</span>
          </div>
        ))}
      </div>
    </div>
  );
}
