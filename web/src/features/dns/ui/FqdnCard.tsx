import { CheckIcon, CopyIcon, ServerIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { cn } from "@/lib/utils";
import { hasSyncStatus, isSynced } from "../domain/dns.types";
import type { Fqdn } from "../domain/dns.types";

interface FqdnCardProps {
  fqdn: Fqdn;
}

export function FqdnCard({ fqdn }: FqdnCardProps) {
  const { copied, copy } = useCopyToClipboard(fqdn.name);

  const sourceLabel = fqdn.source === "manual" ? "Manual" : "External DNS";
  const synced = isSynced(fqdn.syncStatus);
  const syncTooltip = synced
    ? "DNS in sync"
    : fqdn.syncStatus === "notavailable"
      ? "DNS resolution not available"
      : "DNS not in sync";

  const originLabel = fqdn.originRef
    ? `${fqdn.originRef.kind}/${fqdn.originRef.namespace}/${fqdn.originRef.name}`
    : "";

  return (
    <div className="surface surface-interactive min-h-[var(--row-height)] px-3 py-1 flex flex-row items-center gap-2">
      {hasSyncStatus(fqdn.syncStatus) && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span
              aria-label={syncTooltip}
              className={cn(
                "size-2 rounded-full shrink-0 inline-block",
                synced
                  ? "bg-emerald-500 shadow-[0_0_6px_oklch(0.7_0.18_152/0.6)]"
                  : "bg-rose-500 shadow-[0_0_6px_oklch(0.65_0.22_22/0.7)]"
              )}
            />
          </TooltipTrigger>
          <TooltipContent>{syncTooltip}</TooltipContent>
        </Tooltip>
      )}
      <Tooltip>
        <TooltipTrigger asChild>
          <a
            href={`https://${fqdn.name}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-foreground hover:text-primary font-mono text-[13px] font-medium hover:underline underline-offset-4 decoration-primary/40 truncate min-w-0 transition-colors"
          >
            {fqdn.name}
          </a>
        </TooltipTrigger>
        <TooltipContent side="top" align="start" className="max-w-sm">
          <p className="font-mono break-all">{fqdn.name}</p>
          {fqdn.description && (
            <p className="opacity-80">{fqdn.description}</p>
          )}
        </TooltipContent>
      </Tooltip>
      {fqdn.recordType && (
        <Badge
          variant="outline"
          className="shrink-0 font-mono text-[10px] uppercase tracking-wider text-primary border-primary/30 bg-primary/5"
        >
          {fqdn.recordType}
        </Badge>
      )}
      {fqdn.targets.length === 1 && (
        <span
          className="text-muted-foreground font-mono text-xs truncate min-w-0"
          title={fqdn.targets[0]}
        >
          {fqdn.targets[0]}
        </span>
      )}
      {fqdn.targets.length > 1 && (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="inline-flex shrink-0 items-center gap-1 rounded-md border border-transparent px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label={`${fqdn.targets.length} targets`}
            >
              <ServerIcon className="size-3 shrink-0" />
              <span>{fqdn.targets.length} targets</span>
            </button>
          </TooltipTrigger>
          <TooltipContent side="top" align="start">
            <div className="flex flex-col gap-1">
              {fqdn.targets.map((target) => (
                <span key={target} className="font-mono text-[11px]">
                  {target}
                </span>
              ))}
            </div>
          </TooltipContent>
        </Tooltip>
      )}
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge
            variant="secondary"
            className={cn(
              "ml-auto shrink-0 text-[10px] font-mono uppercase tracking-wider cursor-default",
              fqdn.source === "manual" &&
                "bg-amber-500/10 text-amber-700 dark:text-amber-400 border border-amber-500/20"
            )}
          >
            {sourceLabel}
          </Badge>
        </TooltipTrigger>
        <TooltipContent side="top" align="end" className="max-w-sm">
          <p>{sourceLabel}</p>
          {originLabel && (
            <p className="font-mono text-[11px] opacity-80 break-all">
              {originLabel}
            </p>
          )}
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-6 shrink-0"
            onClick={copy}
            aria-label="Copy FQDN to clipboard"
          >
            {copied ? (
              <CheckIcon className="size-3.5 text-green-600" />
            ) : (
              <CopyIcon className="size-3.5" />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent>{copied ? "Copied!" : "Copy FQDN"}</TooltipContent>
      </Tooltip>
    </div>
  );
}
