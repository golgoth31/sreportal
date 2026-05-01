import { CalendarIcon, WrenchIcon } from "lucide-react";

import { cn } from "@/lib/utils";

import type { Maintenance, MaintenancePhase } from "../domain/types";

interface MaintenanceSectionProps {
  maintenances: Maintenance[];
}

export function MaintenanceSection({ maintenances }: MaintenanceSectionProps) {
  if (maintenances.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <CalendarIcon className="size-8 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">
          No maintenances scheduled for this portal.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <h3 className="text-[10px] font-mono uppercase tracking-[0.18em] text-muted-foreground">
        Maintenances
      </h3>
      {maintenances.map((m) => (
        <MaintenanceCard key={m.name} maintenance={m} />
      ))}
    </div>
  );
}

function MaintenanceCard({ maintenance }: { maintenance: Maintenance }) {
  const isActive = maintenance.phase === "in_progress";

  return (
    <div
      className={cn(
        "rounded-lg border p-4 backdrop-blur-sm",
        isActive
          ? "border-primary/40 bg-primary/5"
          : "border-border/70 bg-card/60"
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2 min-w-0">
          <WrenchIcon className="size-4 shrink-0 mt-0.5 text-primary" />
          <div className="min-w-0">
            <h4 className="font-medium text-sm tracking-tight">{maintenance.title}</h4>
            <p className="text-xs text-muted-foreground mt-0.5">
              {formatDateRange(maintenance.scheduledStart, maintenance.scheduledEnd)}
            </p>
            {maintenance.components.length > 0 && (
              <p className="text-xs text-muted-foreground mt-1">
                Affects: {maintenance.components.join(", ")}
              </p>
            )}
          </div>
        </div>
        <PhaseBadge phase={maintenance.phase} />
      </div>
      {maintenance.description && (
        <p className="text-xs text-muted-foreground mt-2 ml-6">
          {maintenance.description}
        </p>
      )}
    </div>
  );
}

function PhaseBadge({ phase }: { phase: MaintenancePhase }) {
  const styles: Record<MaintenancePhase, string> = {
    in_progress:
      "bg-primary/10 text-primary border-primary/30",
    upcoming:
      "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/30",
    completed:
      "bg-muted text-muted-foreground border-border",
  };

  const labels: Record<MaintenancePhase, string> = {
    in_progress: "in progress",
    upcoming: "upcoming",
    completed: "completed",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-mono uppercase tracking-wider whitespace-nowrap",
        styles[phase]
      )}
    >
      {labels[phase]}
    </span>
  );
}

function formatDateRange(start: string, end: string): string {
  if (!start) return "";
  const s = new Date(start);
  const e = new Date(end);
  const opts: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZoneName: "short",
  };
  return `${s.toLocaleString(undefined, opts)} \u2192 ${e.toLocaleString(undefined, opts)}`;
}
