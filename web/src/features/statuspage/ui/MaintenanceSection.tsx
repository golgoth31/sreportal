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
      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
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
        "rounded-lg border p-4",
        isActive
          ? "border-blue-300 bg-blue-50/50 dark:border-blue-700 dark:bg-blue-950/30"
          : "bg-card"
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2 min-w-0">
          <WrenchIcon className="size-4 shrink-0 mt-0.5 text-blue-500" />
          <div className="min-w-0">
            <h4 className="font-medium text-sm">{maintenance.title}</h4>
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
      "bg-blue-100 text-blue-700 border-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-700",
    upcoming:
      "bg-yellow-100 text-yellow-700 border-yellow-200 dark:bg-yellow-900 dark:text-yellow-300 dark:border-yellow-700",
    completed:
      "bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:border-gray-700",
  };

  const labels: Record<MaintenancePhase, string> = {
    in_progress: "IN PROGRESS",
    upcoming: "UPCOMING",
    completed: "COMPLETED",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold whitespace-nowrap",
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
