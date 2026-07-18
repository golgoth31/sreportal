import {
  CheckCircle2Icon,
  ExternalLinkIcon,
  GitCommitIcon,
  GitPullRequestIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { DeployStatusEntry } from "../domain/deploystatus.types";
import { stateBadgeClass, stateLabel } from "./deploystatus.badge-utils";

interface DeployStatusCardProps {
  entry: DeployStatusEntry;
}

function formatRelativeTime(isoString: string | undefined): string {
  if (!isoString) return "—";
  const diff = Date.now() - new Date(isoString).getTime();
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function shortSha(sha: string): string {
  return sha.slice(0, 7);
}

export function DeployStatusCard({ entry }: DeployStatusCardProps) {
  const workloadLabel = entry.workload
    ? `${entry.workload.kind}/${entry.workload.namespace}/${entry.workload.name}`
    : entry.key;

  return (
    <div
      className={cn(
        "rounded-lg border bg-card text-card-foreground shadow-sm p-4 flex flex-col gap-3",
        entry.state === "behind" && "border-amber-300/60 dark:border-amber-700/60",
        entry.state === "error" && "border-red-300/60 dark:border-red-700/60",
      )}
    >
      {/* Header row: workload name + state badge */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-col gap-0.5 min-w-0">
          <span className="font-medium text-sm truncate" title={workloadLabel}>
            {workloadLabel}
          </span>
          <span className="text-xs text-muted-foreground font-mono truncate" title={entry.image}>
            {entry.image}
          </span>
        </div>
        <Badge
          variant="outline"
          className={cn("shrink-0 text-[11px] font-mono", stateBadgeClass(entry.state))}
        >
          {stateLabel(entry.state)}
        </Badge>
      </div>

      {/* Ref info */}
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>
          <span className="font-medium text-foreground">deployed:</span>{" "}
          <span className="font-mono">{shortSha(entry.deployedRef) || entry.deployedRef || "—"}</span>
        </span>
        <span>
          <span className="font-medium text-foreground">branch:</span>{" "}
          <span className="font-mono">{entry.defaultBranch || "—"}</span>
        </span>
        {entry.lastCheckedAt && (
          <span className="ml-auto text-[11px]">
            checked {formatRelativeTime(entry.lastCheckedAt)}
          </span>
        )}
      </div>

      {/* "behind" details */}
      {entry.state === "behind" && (
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium text-amber-700 dark:text-amber-400">
              {entry.aheadBy} commit{entry.aheadBy !== 1 ? "s" : ""} behind
            </span>
            {entry.deployRunUrl && (
              <a
                href={entry.deployRunUrl}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
              >
                <GitPullRequestIcon className="size-3" aria-hidden="true" />
                Deploy to prod
                <ExternalLinkIcon className="size-2.5" aria-hidden="true" />
              </a>
            )}
          </div>

          {entry.pendingCommits.length > 0 && (
            <ul className="flex flex-col gap-1">
              {entry.pendingCommits.map((c) => (
                <li key={c.sha} className="flex items-start gap-1.5 text-xs">
                  <GitCommitIcon
                    className="size-3 mt-0.5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <code className="font-mono text-[11px] text-muted-foreground shrink-0">
                    {c.url ? (
                      <a
                        href={c.url}
                        target="_blank"
                        rel="noreferrer"
                        className="hover:underline"
                      >
                        {shortSha(c.sha)}
                      </a>
                    ) : (
                      shortSha(c.sha)
                    )}
                  </code>
                  <span className="truncate" title={c.message}>
                    {c.message}
                  </span>
                </li>
              ))}
            </ul>
          )}

          {entry.pendingTruncated && (
            <p className="text-xs text-muted-foreground italic">
              …and more commits not shown.{" "}
              {entry.sourceRepo && (
                <a
                  href={`${entry.sourceRepo}/compare/${shortSha(entry.deployedRef)}...${entry.defaultBranch}`}
                  target="_blank"
                  rel="noreferrer"
                  className="text-primary hover:underline inline-flex items-center gap-0.5"
                >
                  View full diff
                  <ExternalLinkIcon className="size-2.5" aria-hidden="true" />
                </a>
              )}
            </p>
          )}
        </div>
      )}

      {/* error detail */}
      {entry.state === "error" && entry.error && (
        <p className="text-xs text-red-700 dark:text-red-400 font-mono break-all">
          {entry.error}
        </p>
      )}

      {/* ok check */}
      {entry.state === "ok" && (
        <div className="flex items-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400">
          <CheckCircle2Icon className="size-3.5" aria-hidden="true" />
          All changes deployed
        </div>
      )}
    </div>
  );
}
