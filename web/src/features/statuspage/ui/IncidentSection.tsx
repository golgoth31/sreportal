import { useState } from "react";
import { ChevronDownIcon, ChevronRightIcon } from "lucide-react";

import type { Incident, IncidentPhase, IncidentSeverity } from "../domain/types";

interface IncidentSectionProps {
  incidents: Incident[];
}

export function IncidentSection({ incidents }: IncidentSectionProps) {
  if (incidents.length === 0) return null;

  const active = incidents.filter((i) => i.currentPhase !== "resolved");
  const resolved = incidents
    .filter((i) => i.currentPhase === "resolved")
    .slice(0, 10);

  return (
    <div className="space-y-3">
      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        Incidents
      </h3>
      {active.map((inc) => (
        <IncidentCard key={inc.name} incident={inc} defaultOpen />
      ))}
      {resolved.map((inc) => (
        <IncidentCard key={inc.name} incident={inc} defaultOpen={false} />
      ))}
    </div>
  );
}

function IncidentCard({
  incident,
  defaultOpen,
}: {
  incident: Incident;
  defaultOpen: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const isActive = incident.currentPhase !== "resolved";
  const severityColor = getSeverityColor(incident.severity);

  return (
    <div
      className={`rounded-lg border p-4 ${
        isActive
          ? "border-red-300 bg-red-50/50 dark:border-red-700 dark:bg-red-950/30"
          : "bg-card"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h4 className="font-medium text-sm">{incident.title}</h4>
            <span
              className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold ${severityColor}`}
            >
              {incident.severity.toUpperCase()}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            {isActive
              ? `Since ${formatDate(incident.startedAt)}`
              : `${formatDate(incident.startedAt)} \u2192 ${formatDate(incident.resolvedAt)} \u00B7 ${incident.durationMinutes} min`}
          </p>
          {incident.components.length > 0 && (
            <p className="text-xs text-muted-foreground mt-0.5">
              Affects: {incident.components.join(", ")}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <IncidentPhaseBadge phase={incident.currentPhase} />
          <button
            onClick={() => setOpen(!open)}
            className="p-1 hover:bg-muted rounded"
            aria-label={open ? "Collapse timeline" : "Expand timeline"}
          >
            {open ? (
              <ChevronDownIcon className="size-4" />
            ) : (
              <ChevronRightIcon className="size-4" />
            )}
          </button>
        </div>
      </div>
      {open && incident.updates.length > 0 && (
        <div className="mt-3 ml-1 border-l-2 border-muted pl-4 space-y-2">
          {incident.updates.map((update, i) => (
            <div key={i} className="text-xs">
              <span className="text-muted-foreground">
                {formatTime(update.timestamp)}
              </span>
              <span className="mx-1.5 font-semibold uppercase text-[10px]">
                [{update.phase}]
              </span>
              <span>{update.message}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function IncidentPhaseBadge({ phase }: { phase: IncidentPhase }) {
  const styles: Record<IncidentPhase, string> = {
    investigating:
      "bg-red-100 text-red-700 border-red-200 dark:bg-red-900 dark:text-red-300",
    identified:
      "bg-orange-100 text-orange-700 border-orange-200 dark:bg-orange-900 dark:text-orange-300",
    monitoring:
      "bg-yellow-100 text-yellow-700 border-yellow-200 dark:bg-yellow-900 dark:text-yellow-300",
    resolved:
      "bg-green-100 text-green-700 border-green-200 dark:bg-green-900 dark:text-green-300",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold whitespace-nowrap ${styles[phase]}`}
    >
      {phase.toUpperCase()}
    </span>
  );
}

function getSeverityColor(severity: IncidentSeverity): string {
  switch (severity) {
    case "critical":
      return "bg-red-100 text-red-700 border-red-200 dark:bg-red-900 dark:text-red-300";
    case "major":
      return "bg-orange-100 text-orange-700 border-orange-200 dark:bg-orange-900 dark:text-orange-300";
    case "minor":
      return "bg-yellow-100 text-yellow-700 border-yellow-200 dark:bg-yellow-900 dark:text-yellow-300";
  }
}

function formatDate(iso: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatTime(iso: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}
