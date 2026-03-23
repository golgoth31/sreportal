import { useMemo, useState, useCallback } from "react";

import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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

const TYPE_COLORS: Record<string, string> = {
  service: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
  database: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
  messaging: "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300",
  cron: "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
  external: "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
};

interface Props {
  nodes: readonly NetpolNode[];
  nodeMap: ReadonlyMap<string, NetpolNode>;
  callsTo: ReadonlyMap<string, readonly NetpolEdge[]>;
  callsFrom: ReadonlyMap<string, readonly NetpolEdge[]>;
  allGroups: readonly string[];
}

export function FlowMatrixView({ nodes, nodeMap, callsTo, callsFrom, allGroups }: Props) {
  const [search, setSearch] = useState("");
  const [nsFilter, setNsFilter] = useState("");
  const [collapsedNs, setCollapsedNs] = useState<Set<string>>(new Set());
  const [collapsedSvc, setCollapsedSvc] = useState<Set<string>>(new Set());

  const toggleNs = useCallback((ns: string) => {
    setCollapsedNs((prev) => {
      const next = new Set(prev);
      if (next.has(ns)) next.delete(ns); else next.add(ns);
      return next;
    });
  }, []);

  const toggleSvc = useCallback((id: string) => {
    setCollapsedSvc((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const services = useMemo(() => {
    let filtered = nodes.filter((n) => n.nodeType === "service");
    if (nsFilter) filtered = filtered.filter((n) => n.group === nsFilter);
    if (search) {
      const q = search.toLowerCase();
      filtered = filtered.filter((n) => n.label.toLowerCase().includes(q));
    }
    return filtered.sort((a, b) => a.group.localeCompare(b.group) || a.label.localeCompare(b.label));
  }, [nodes, search, nsFilter]);

  const byNs = useMemo(() => {
    const map = new Map<string, NetpolNode[]>();
    for (const s of services) {
      const arr = map.get(s.group) ?? [];
      arr.push(s);
      map.set(s.group, arr);
    }
    return map;
  }, [services]);

  const copyMarkdown = () => {
    const lines: string[] = [
      "# Network Flow Matrix",
      "",
      `Date: ${new Date().toISOString().slice(0, 10)}`,
      "",
    ];

    for (const [ns, svcs] of [...byNs.entries()].sort(([a], [b]) => a.localeCompare(b))) {
      lines.push(`## ${ns}`, "");
      for (const svc of svcs) {
        lines.push(`### ${svc.label}`, "");
        const out = dedup(callsTo.get(svc.id) ?? []);
        const inb = dedup(callsFrom.get(svc.id) ?? []);

        lines.push(`**Calls to (${out.length})**`);
        if (out.length) {
          for (const e of out) {
            const n = nodeMap.get(e.to === svc.id ? e.from : e.to);
            if (!n) continue;
            const xns = n.group !== svc.group ? ` → *${n.group}*` : "";
            lines.push(`- \`${n.nodeType}\` **${n.label}**${xns} · ${e.edgeType}`);
          }
        } else {
          lines.push("- *none*");
        }
        lines.push("");

        lines.push(`**Called from (${inb.length})**`);
        if (inb.length) {
          for (const e of inb) {
            const n = nodeMap.get(e.from);
            if (!n) continue;
            const xns = n.group !== svc.group ? ` → *${n.group}*` : "";
            lines.push(`- \`${n.nodeType}\` **${n.label}**${xns} · ${e.edgeType}`);
          }
        } else {
          lines.push("- *none*");
        }
        lines.push("", "---", "");
      }
    }

    navigator.clipboard.writeText(lines.join("\n"));
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <Input
          placeholder="Filter services..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
        />
        <Select value={nsFilter || "__all__"} onValueChange={(v) => setNsFilter(v === "__all__" ? "" : v)}>
          <SelectTrigger className="w-48">
            <SelectValue placeholder="All namespaces" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All namespaces</SelectItem>
            {allGroups.map((g) => (
              <SelectItem key={g} value={g}>{g}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <button
          onClick={copyMarkdown}
          className="text-xs px-3 py-1.5 rounded-md border border-border bg-muted hover:bg-accent transition-colors"
        >
          Copy as Markdown
        </button>
        <span className="text-muted-foreground text-sm ml-auto">
          {services.length} services across {byNs.size} namespaces
        </span>
      </div>

      {[...byNs.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([ns, svcs]) => {
        const nsCollapsed = collapsedNs.has(ns);
        return (
          <div key={ns} className="space-y-2">
            <button
              onClick={() => toggleNs(ns)}
              className="flex items-center gap-2 w-full text-left text-lg font-semibold border-b pb-1 hover:text-accent-foreground transition-colors"
            >
              <span className="text-muted-foreground text-sm">{nsCollapsed ? "▶" : "▼"}</span>
              {ns}
              <span className="text-muted-foreground text-sm font-normal ml-1">({svcs.length})</span>
            </button>
            {!nsCollapsed && (
              <div className="space-y-2 ml-2">
                {svcs.map((svc) => {
                  const svcCollapsed = collapsedSvc.has(svc.id);
                  return (
                    <ServiceCard
                      key={svc.id}
                      svc={svc}
                      collapsed={svcCollapsed}
                      onToggle={() => toggleSvc(svc.id)}
                      callsTo={dedup(callsTo.get(svc.id) ?? [])}
                      calledFrom={dedup(callsFrom.get(svc.id) ?? [])}
                      nodeMap={nodeMap}
                    />
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function dedup(edges: readonly NetpolEdge[]): NetpolEdge[] {
  const seen = new Set<string>();
  return edges.filter((e) => {
    const key = `${e.from}|${e.to}|${e.edgeType}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function ServiceCard({
  svc,
  collapsed,
  onToggle,
  callsTo,
  calledFrom,
  nodeMap,
}: {
  svc: NetpolNode;
  collapsed: boolean;
  onToggle: () => void;
  callsTo: readonly NetpolEdge[];
  calledFrom: readonly NetpolEdge[];
  nodeMap: ReadonlyMap<string, NetpolNode>;
}) {
  return (
    <div className="rounded-lg border bg-card overflow-hidden">
      <button
        onClick={onToggle}
        className="flex items-center gap-2 w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors"
      >
        <span className="text-muted-foreground text-xs">{collapsed ? "▶" : "▼"}</span>
        <span className="font-medium">{svc.label}</span>
        <Badge variant="outline" className={groupColor(svc.group)}>{svc.group}</Badge>
        <span className="text-muted-foreground text-xs ml-auto">
          {callsTo.length} out · {calledFrom.length} in
        </span>
      </button>

      {!collapsed && (
        <div className="px-4 pb-4 space-y-3">
          <div>
            <p className="text-xs font-medium text-green-600 dark:text-green-400 uppercase tracking-wide mb-1">
              Calls to ({callsTo.length})
            </p>
            {callsTo.length === 0 && <p className="text-xs text-muted-foreground pl-3 italic">No outgoing flows</p>}
            {callsTo.map((e, i) => {
              const n = nodeMap.get(e.to);
              if (!n) return null;
              return <FlowRow key={i} node={n} edgeType={e.edgeType} svcGroup={svc.group} />;
            })}
          </div>

          <div>
            <p className="text-xs font-medium text-blue-600 dark:text-blue-400 uppercase tracking-wide mb-1">
              Called from ({calledFrom.length})
            </p>
            {calledFrom.length === 0 && <p className="text-xs text-muted-foreground pl-3 italic">No incoming flows</p>}
            {calledFrom.map((e, i) => {
              const n = nodeMap.get(e.from);
              if (!n) return null;
              return <FlowRow key={i} node={n} edgeType={e.edgeType} svcGroup={svc.group} />;
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function FlowRow({ node, edgeType, svcGroup }: { node: NetpolNode; edgeType: string; svcGroup: string }) {
  return (
    <div className="flex items-center gap-2 pl-3 py-0.5 text-sm">
      <Badge variant="outline" className={TYPE_COLORS[node.nodeType] ?? ""}>
        {node.nodeType}
      </Badge>
      <span className="font-medium">{node.label}</span>
      {node.group !== svcGroup && (
        <Badge variant="outline" className="text-[10px] bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300">
          {node.group}
        </Badge>
      )}
      <span className="text-muted-foreground text-xs">{edgeType}</span>
    </div>
  );
}
