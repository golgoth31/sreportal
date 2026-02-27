import { CheckIcon, CopyIcon, ExternalLinkIcon, NetworkIcon } from "lucide-react";
import { useCallback, useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { hasSyncStatus, isSynced } from "../domain/dns.types";
import type { Fqdn } from "../domain/dns.types";

interface FqdnCardProps {
  fqdn: Fqdn;
}

export function FqdnCard({ fqdn }: FqdnCardProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(fqdn.name);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  }, [fqdn.name]);

  const sourceLabel = fqdn.source === "manual" ? "Manual" : "External DNS";
  const synced = isSynced(fqdn.syncStatus);
  const syncTooltip = synced
    ? "DNS in sync"
    : fqdn.syncStatus === "notavailable"
      ? "DNS resolution not available"
      : "DNS not in sync";

  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-3 shadow-xs">
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
                    synced ? "bg-green-500" : "bg-red-500"
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
            className="text-primary font-mono text-sm font-medium hover:underline break-all flex items-center gap-1"
          >
            {fqdn.name}
            <ExternalLinkIcon className="size-3 shrink-0 text-muted-foreground" />
          </a>
        </div>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7 shrink-0"
              onClick={handleCopy}
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
        <p className="text-muted-foreground text-xs">{fqdn.description}</p>
      )}

      {/* Meta: record type + source */}
      <div className="flex flex-wrap items-center gap-1.5">
        {fqdn.recordType && (
          <Badge variant="outline" className="font-mono text-xs">
            {fqdn.recordType}
          </Badge>
        )}
        <Badge variant="secondary" className="text-xs">
          {sourceLabel}
        </Badge>
        {fqdn.targets.map((target) => (
          <span
            key={target}
            className="text-muted-foreground font-mono text-xs"
          >
            → {target}
          </span>
        ))}
      </div>

      {/* Origin resource reference */}
      {fqdn.originRef && (
        <div className="border-t pt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
          <NetworkIcon className="size-3.5 shrink-0" />
          <span className="font-mono">
            {fqdn.originRef.kind}/{fqdn.originRef.namespace}/{fqdn.originRef.name}
          </span>
        </div>
      )}
    </div>
  );
}
