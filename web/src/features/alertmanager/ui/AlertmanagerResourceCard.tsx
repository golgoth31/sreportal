import {
  ChevronDownIcon,
  ExternalLinkIcon,
  AlertTriangleIcon,
} from "lucide-react";
import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import type { AlertmanagerResource } from "../domain/alertmanager.types";
import { groupAlertsByName } from "../domain/alertmanager.types";
import { AlertGroupCard } from "./AlertGroupCard";

interface AlertmanagerResourceCardProps {
  resource: AlertmanagerResource;
}

export function AlertmanagerResourceCard({ resource }: AlertmanagerResourceCardProps) {
  const [open, setOpen] = useState(true);
  const alertCount = resource.alerts.length;
  const displayUrl = resource.remoteUrl || resource.localUrl;

  const alertGroups = useMemo(
    () => groupAlertsByName(resource.alerts),
    [resource.alerts]
  );

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <div className="rounded-lg border border-border/70 bg-card/40 backdrop-blur-sm overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button
            variant="ghost"
            className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/40 bg-gradient-to-r from-primary/[0.04] to-transparent"
          >
            <div className="flex items-center gap-3 flex-wrap">
              <AlertTriangleIcon className="size-4 text-primary/70 shrink-0" />
              <span className="font-display italic text-base text-foreground tracking-tight">
                {resource.name}
              </span>
              <span className="text-muted-foreground text-[11px] font-mono">
                {resource.namespace}
              </span>
              <span className="text-muted-foreground text-[11px] font-mono uppercase tracking-wider px-2 py-0.5 rounded-full bg-muted/60">
                {alertCount} {alertCount === 1 ? "alert" : "alerts"}
              </span>
              {resource.ready ? (
                <Badge
                  variant="secondary"
                  className="text-[10px] font-mono uppercase tracking-wider bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-500/20"
                >
                  Ready
                </Badge>
              ) : (
                <Badge
                  variant="outline"
                  className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground"
                >
                  Not ready
                </Badge>
              )}
            </div>
            <ChevronDownIcon
              className={cn(
                "size-4 text-muted-foreground transition-transform duration-200 shrink-0",
                open && "rotate-180"
              )}
            />
          </Button>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="border-t px-4 pb-4 pt-2">
            {displayUrl && (
              <p className="text-xs text-muted-foreground mb-3 flex items-center gap-1">
                <a
                  href={displayUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline inline-flex items-center gap-1"
                >
                  {resource.remoteUrl ? "Remote" : "Local"} Alertmanager
                  <ExternalLinkIcon className="size-3" />
                </a>
              </p>
            )}
            {resource.alerts.length === 0 ? (
              <p className="text-sm text-muted-foreground py-2">
                No active alerts.
              </p>
            ) : (
              <div className="space-y-1">
                {alertGroups.map((group) => (
                  <AlertGroupCard key={group.name} group={group} />
                ))}
              </div>
            )}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
