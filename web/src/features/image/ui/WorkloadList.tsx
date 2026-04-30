import { BoxIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import type { WorkloadRef } from "../domain/image.types";

interface WorkloadListProps {
  workloads: readonly WorkloadRef[];
  variant?: "compact" | "full";
  className?: string;
}

export function groupWorkloadsByNamespace(
  workloads: readonly WorkloadRef[],
): { namespace: string; items: WorkloadRef[] }[] {
  const map = new Map<string, WorkloadRef[]>();
  for (const w of workloads) {
    if (!map.has(w.namespace)) map.set(w.namespace, []);
    map.get(w.namespace)!.push(w);
  }
  return Array.from(map.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([namespace, items]) => ({
      namespace,
      items: [...items].sort((a, b) =>
        `${a.kind}/${a.name}`.localeCompare(`${b.kind}/${b.name}`),
      ),
    }));
}

export function WorkloadList({
  workloads,
  variant = "full",
  className,
}: WorkloadListProps) {
  if (variant === "compact") {
    return (
      <ul className={cn("flex flex-col gap-1", className)}>
        {workloads.map((w, i) => (
          <li
            key={`${w.kind}/${w.namespace}/${w.name}/${w.container}/${i}`}
            className="flex items-center gap-1.5 font-mono text-[11px] leading-tight"
          >
            <BoxIcon className="size-3 shrink-0 opacity-60" />
            <span className="truncate">
              <span className="opacity-70">{w.kind}</span>{" "}
              <span>{w.namespace}/{w.name}</span>
            </span>
          </li>
        ))}
      </ul>
    );
  }

  const groups = groupWorkloadsByNamespace(workloads);
  return (
    <div className={cn("flex flex-col gap-4", className)}>
      {groups.map((group) => (
        <section key={group.namespace} className="flex flex-col gap-2">
          <header className="flex items-baseline justify-between gap-2 border-b pb-1">
            <h3 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              {group.namespace}
            </h3>
            <span className="text-[10px] text-muted-foreground/70">
              {group.items.length}
            </span>
          </header>
          <ul className="flex flex-col gap-1.5">
            {group.items.map((w, i) => (
              <li
                key={`${w.kind}/${w.name}/${w.container}/${i}`}
                className="flex items-start gap-2 rounded-md border bg-card/50 px-2.5 py-1.5"
              >
                <BoxIcon className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5 text-xs">
                    <span className="rounded bg-muted px-1 py-0.5 font-mono text-[10px] uppercase tracking-wide text-muted-foreground">
                      {w.kind}
                    </span>
                    <span className="truncate font-mono text-xs">{w.name}</span>
                  </div>
                  <p className="mt-0.5 truncate font-mono text-[10px] text-muted-foreground">
                    container: {w.container}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
