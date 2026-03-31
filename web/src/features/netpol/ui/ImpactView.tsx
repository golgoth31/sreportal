import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { computeImpact, type NetpolNode, type NetpolEdge, type ImpactLevel } from "../domain/netpol.types";
import { NODE_TYPE_COLORS, MAX_SEARCH_RESULTS } from "../domain/utils";

const DEPTH_COLORS = [
  "border-l-red-500",
  "border-l-orange-500",
  "border-l-yellow-500",
  "border-l-green-500",
  "border-l-blue-500",
  "border-l-purple-500",
];

const DEPTH_BADGE = [
  "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
];

const DEPTH_LABELS = [
  "Direct resource",
  "Level 1 — Direct dependents",
  "Level 2 — Indirect dependents",
  "Level 3",
  "Level 4",
  "Level 5",
];

interface Props {
  nodes: readonly NetpolNode[];
  nodeMap: ReadonlyMap<string, NetpolNode>;
  callsFrom: ReadonlyMap<string, readonly NetpolEdge[]>;
}

export function ImpactView({ nodes, nodeMap, callsFrom }: Props) {
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState("");

  const filteredNodes = useMemo(() => {
    if (!search) return [];
    const q = search.toLowerCase();
    return nodes
      .filter((n) => n.label.toLowerCase().includes(q) || n.group.toLowerCase().includes(q))
      .sort((a, b) => a.nodeType.localeCompare(b.nodeType) || a.label.localeCompare(b.label))
      .slice(0, MAX_SEARCH_RESULTS);
  }, [nodes, search]);

  const levels = useMemo((): ImpactLevel[] => {
    if (!selectedId) return [];
    return computeImpact(selectedId, nodeMap, callsFrom);
  }, [selectedId, nodeMap, callsFrom]);

  const blastRadius = useMemo(
    () => levels.reduce((sum, l) => sum + l.nodes.length, 0) - 1,
    [levels]
  );

  const selectedNode = selectedId ? nodeMap.get(selectedId) : undefined;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold mb-1">Impact Analysis</h2>
        <p className="text-sm text-muted-foreground mb-3">
          Select a resource (database, service, external endpoint) to see the full blast radius.
        </p>
      </div>

      <Input
        placeholder="Search a resource..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="max-w-md"
      />

      {search && filteredNodes.length > 0 && (
        <div className="grid gap-1 max-h-48 overflow-y-auto" role="listbox" aria-label="Search results">
          {filteredNodes.map((n) => (
            <button
              key={n.id}
              role="option"
              aria-selected={n.id === selectedId}
              onClick={() => { setSelectedId(n.id); setSearch(""); }}
              className={cn(
                "flex items-center gap-2 px-3 py-1.5 rounded-md text-sm text-left transition-colors",
                n.id === selectedId ? "bg-accent" : "hover:bg-muted"
              )}
            >
              <Badge variant="outline" className={cn(NODE_TYPE_COLORS[n.nodeType])}>
                {n.nodeType}
              </Badge>
              <span className="font-medium">{n.label}</span>
              <span className="text-muted-foreground text-xs">{n.group}</span>
            </button>
          ))}
        </div>
      )}

      {selectedNode && (
        <div className="rounded-lg border bg-muted/50 p-3 text-sm space-y-1">
          <p>
            <span className="font-semibold">{selectedNode.label}</span>
            <span className="text-muted-foreground"> ({selectedNode.nodeType} · {selectedNode.group})</span>
          </p>
          <p className="text-muted-foreground">
            Total blast radius: <span className="font-semibold text-foreground">{blastRadius}</span> resources impacted
            across <span className="font-semibold text-foreground">{levels.length - 1}</span> levels
          </p>
        </div>
      )}

      {levels.map((level) => (
        <div
          key={level.depth}
          className={cn(
            "rounded-lg border bg-card p-4 border-l-4",
            DEPTH_COLORS[Math.min(level.depth, DEPTH_COLORS.length - 1)]
          )}
        >
          <div className="flex items-center gap-2 mb-2">
            <span className="font-medium text-sm">
              {DEPTH_LABELS[level.depth] ?? `Level ${level.depth}`}
            </span>
            <Badge
              variant="outline"
              className={cn(DEPTH_BADGE[Math.min(level.depth, DEPTH_BADGE.length - 1)])}
            >
              {level.nodes.length}
            </Badge>
          </div>
          <div className="space-y-1">
            {level.nodes.map((item) => (
              <div key={item.node.id} className="flex items-center gap-2 text-sm">
                <Badge variant="outline" className={cn(NODE_TYPE_COLORS[item.node.nodeType])}>
                  {item.node.nodeType}
                </Badge>
                <span className="font-medium">{item.node.label}</span>
                <span className="text-muted-foreground text-xs">{item.node.group}</span>
                {item.via && (
                  <span className="text-muted-foreground text-xs">← via {item.via}</span>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
