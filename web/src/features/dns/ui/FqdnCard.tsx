import { CheckIcon, CopyIcon, NetworkIcon } from "lucide-react";

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

  return (
    <div className="group rounded-lg border border-border/70 bg-card/60 backdrop-blur-sm p-4 flex flex-col gap-3 transition-all hover:border-primary/40 hover:bg-card hover:shadow-md hover:shadow-primary/5">
      {/* FQDN name + sync dot + copy */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
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
          <a
            href={`https://${fqdn.name}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-foreground hover:text-primary font-mono text-[13px] font-medium hover:underline underline-offset-4 decoration-primary/40 break-all flex items-center gap-1 transition-colors"
          >
            {fqdn.name}
          </a>
        </div>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7 shrink-0"
              onClick={copy}
              aria-label="Copy FQDN to clipboard"
            >
              {copied ? (
                <CheckIcon className="size-4 text-green-600" />
              ) : (
                <CopyIcon className="size-4" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{copied ? "Copied!" : "Copy FQDN"}</TooltipContent>
        </Tooltip>
      </div>

      {/* Description */}
      {fqdn.description && (
        <p className="text-muted-foreground text-xs leading-relaxed">{fqdn.description}</p>
      )}

      {/* Meta: record type + source */}
      <div className="flex flex-wrap items-center gap-1.5">
        {fqdn.recordType && (
          <Badge variant="outline" className="font-mono text-[10px] uppercase tracking-wider text-primary border-primary/30 bg-primary/5">
            {fqdn.recordType}
          </Badge>
        )}
        {fqdn.targets.map((target) => (
          <span
            key={target}
            className="text-muted-foreground font-mono text-xs"
          >
            {target}
          </span>
        ))}
        <Badge
          variant="secondary"
          className={cn(
            "text-[10px] font-mono uppercase tracking-wider",
            fqdn.source === "manual" &&
              "bg-amber-500/10 text-amber-700 dark:text-amber-400 border border-amber-500/20"
          )}
        >
          {sourceLabel}
        </Badge>
      </div>

      {/* Origin resource reference */}
      {fqdn.originRef && (
        <div className="border-t border-border/60 pt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
          <NetworkIcon className="size-3.5 shrink-0" />
          <span className="font-mono text-[11px]">
            {fqdn.originRef.kind}/{fqdn.originRef.namespace}/{fqdn.originRef.name}
          </span>
        </div>
      )}
    </div>
  );
}
