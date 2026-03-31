import { useMemo } from "react";
import { GitBranchIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { NetpolNode, NetpolEdge } from "../domain/netpol.types";
import { groupColor, MAX_TOP_FLOWS } from "../domain/utils";

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

  if (flows.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <GitBranchIcon className="size-8 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">
          No cross-namespace flows detected.
        </p>
      </div>
    );
  }

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
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="text-xs">From ↓ / To →</TableHead>
              {allPls.map((pl) => (
                <TableHead key={pl} className="text-center">
                  <Badge variant="outline" className={groupColor(pl)}>{pl}</Badge>
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {allPls.map((srcPl) => (
              <TableRow key={srcPl}>
                <TableCell>
                  <Badge variant="outline" className={groupColor(srcPl)}>{srcPl}</Badge>
                </TableCell>
                {allPls.map((tgtPl) => {
                  const count = matrix.get(srcPl)?.get(tgtPl) ?? 0;
                  return (
                    <TableCell key={tgtPl} className="text-center">
                      {srcPl === tgtPl ? (
                        <span className="text-muted-foreground">—</span>
                      ) : count > 0 ? (
                        <span className="font-medium">{count}</span>
                      ) : (
                        <span className="text-muted-foreground/30">0</span>
                      )}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Top flows */}
      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Top flows
        </h3>
        {flows.slice(0, MAX_TOP_FLOWS).map((f) => (
          <div key={`${f.sourcePl}→${f.targetPl}`} className="flex items-center gap-2 text-sm">
            <Badge variant="outline" className={cn(groupColor(f.sourcePl))}>{f.sourcePl}</Badge>
            <span className="text-muted-foreground">→</span>
            <Badge variant="outline" className={cn(groupColor(f.targetPl))}>{f.targetPl}</Badge>
            <span className="font-medium">{f.count} flows</span>
          </div>
        ))}
      </div>
    </div>
  );
}
