import { ExternalLinkIcon } from "lucide-react";

import type { PlatformComponent } from "../domain/types";
import { getStatusColor, getStatusLabel } from "../domain/utils";
import { DailyStatusBar } from "./DailyStatusBar";

function safeHostname(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

interface ComponentCardProps {
  component: PlatformComponent;
}

export function ComponentCard({ component }: ComponentCardProps) {
  const status = component.computedStatus || component.declaredStatus;
  const colorClass = getStatusColor(status);
  const label = getStatusLabel(status);

  return (
    <div className="group rounded-lg border border-border/70 bg-card/60 backdrop-blur-sm p-4 transition-all hover:border-primary/40 hover:bg-card hover:shadow-md hover:shadow-primary/5">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <h4 className="font-medium text-sm truncate tracking-tight">
            {component.displayName}
          </h4>
          {component.description && (
            <p className="text-xs text-muted-foreground mt-0.5 truncate">
              {component.description}
            </p>
          )}
        </div>
        <span
          className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-mono uppercase tracking-wider whitespace-nowrap ${colorClass}`}
        >
          {label}
        </span>
      </div>
      {component.link && (
        <a
          href={component.link}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 mt-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ExternalLinkIcon className="size-3" />
          <span className="truncate max-w-48">
            {safeHostname(component.link)}
          </span>
        </a>
      )}
      <DailyStatusBar dailyStatus={component.dailyWorstStatus} />
    </div>
  );
}
