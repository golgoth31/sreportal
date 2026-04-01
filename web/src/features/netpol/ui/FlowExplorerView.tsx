import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { NetpolNode, NetpolEdge } from "../domain/netpol.types";
import { NODE_TYPE_COLORS, MAX_SEARCH_RESULTS, groupColor } from "../domain/utils";

interface Props {
  nodes: readonly NetpolNode[];
  nodeMap: ReadonlyMap<string, NetpolNode>;
  callsTo: ReadonlyMap<string, readonly NetpolEdge[]>;
  callsFrom: ReadonlyMap<string, readonly NetpolEdge[]>;
}

interface FlowItem {
  node: NetpolNode;
  edgeType: string;
}

function groupByNamespace(items: readonly FlowItem[]): Map<string, FlowItem[]> {
  const groups = new Map<string, FlowItem[]>();
  for (const item of items) {
    const ns = item.node.group;
    const list = groups.get(ns) ?? [];
    list.push(item);
    groups.set(ns, list);
  }
  return new Map([...groups.entries()].sort(([a], [b]) => a.localeCompare(b)));
}

function NamespaceGroup({
  namespace,
  items,
  onSelect,
}: {
  namespace: string;
  items: FlowItem[];
  onSelect: (id: string) => void;
}) {
  const [open, setOpen] = useState(true);

  return (
    <div className="rounded-lg border bg-card">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
      >
        {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
        <Badge variant="outline" className={cn(groupColor(namespace), "text-[10px]")}>
          {namespace}
        </Badge>
        <span className="text-muted-foreground text-xs ml-auto">{items.length}</span>
      </button>
      {open && (
        <div className="border-t">
          {items.map((item) => (
            <button
              key={`${item.node.id}-${item.edgeType}`}
              onClick={() => onSelect(item.node.id)}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-left transition-colors hover:bg-muted w-full"
            >
              <Badge variant="outline" className={cn(NODE_TYPE_COLORS[item.node.nodeType], "text-[10px]")}>
                {item.node.nodeType}
              </Badge>
              <span className="font-medium truncate">{item.node.label}</span>
              <Badge variant="outline" className="ml-auto text-[10px] shrink-0">
                {item.edgeType}
              </Badge>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function FlowColumn({
  title,
  items,
  onSelect,
  emptyLabel,
}: {
  title: string;
  items: readonly FlowItem[];
  onSelect: (id: string) => void;
  emptyLabel: string;
}) {
  const grouped = useMemo(() => groupByNamespace(items), [items]);

  return (
    <div className="flex-1 min-w-0">
      <h3 className="text-sm font-medium text-muted-foreground mb-2">{title}</h3>
      {items.length === 0 ? (
        <p className="text-xs text-muted-foreground italic px-3 py-4">{emptyLabel}</p>
      ) : (
        <div className="space-y-2 max-h-[60vh] overflow-y-auto">
          {[...grouped.entries()].map(([ns, groupItems]) => (
            <NamespaceGroup
              key={ns}
              namespace={ns}
              items={groupItems}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function FlowExplorerView({ nodes, nodeMap, callsTo, callsFrom }: Props) {
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

  const selectedNode = selectedId ? nodeMap.get(selectedId) : undefined;

  const incoming = useMemo(() => {
    if (!selectedId) return [];
    return (callsFrom.get(selectedId) ?? [])
      .map((e) => ({ node: nodeMap.get(e.from)!, edgeType: e.edgeType }))
      .filter((item) => item.node != null)
      .sort((a, b) => a.node.label.localeCompare(b.node.label));
  }, [selectedId, callsFrom, nodeMap]);

  const outgoing = useMemo(() => {
    if (!selectedId) return [];
    return (callsTo.get(selectedId) ?? [])
      .map((e) => ({ node: nodeMap.get(e.to)!, edgeType: e.edgeType }))
      .filter((item) => item.node != null)
      .sort((a, b) => a.node.label.localeCompare(b.node.label));
  }, [selectedId, callsTo, nodeMap]);

  const handleSelect = (id: string) => {
    setSelectedId(id);
    setSearch("");
  };

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold mb-1">Flow Explorer</h2>
        <p className="text-sm text-muted-foreground mb-3">
          Select a service or resource to see its incoming and outgoing flows. Click on any connected service to navigate to it.
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
              onClick={() => handleSelect(n.id)}
              className={cn(
                "flex items-center gap-2 px-3 py-1.5 rounded-md text-sm text-left transition-colors",
                n.id === selectedId ? "bg-accent" : "hover:bg-muted",
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
        <div className="flex gap-4 items-stretch">
          <FlowColumn
            title={`Incoming (${incoming.length})`}
            items={incoming}
            onSelect={handleSelect}
            emptyLabel="No incoming flows"
          />

          <div className="flex flex-col items-center justify-center px-6 py-8 shrink-0">
            <div className="rounded-xl border-2 border-primary bg-primary/5 p-6 text-center space-y-2 min-w-[180px]">
              <Badge variant="outline" className={cn(NODE_TYPE_COLORS[selectedNode.nodeType], "text-xs")}>
                {selectedNode.nodeType}
              </Badge>
              <p className="font-semibold text-lg">{selectedNode.label}</p>
              <p className="text-muted-foreground text-xs">{selectedNode.group}</p>
            </div>
            <div className="mt-3 text-xs text-muted-foreground">
              {incoming.length} in · {outgoing.length} out
            </div>
          </div>

          <FlowColumn
            title={`Outgoing (${outgoing.length})`}
            items={outgoing}
            onSelect={handleSelect}
            emptyLabel="No outgoing flows"
          />
        </div>
      )}
    </div>
  );
}
