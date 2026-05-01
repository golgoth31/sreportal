import { ChevronRightIcon } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TruncatedCopyTooltipText } from "@/components/TruncatedCopyTooltipText";
import { cn } from "@/lib/utils";
import type { AlertGroup } from "../domain/alertmanager.types";
import {
  formatAlertTime,
  isSilenced,
} from "../domain/alertmanager.types";

interface AlertGroupCardProps {
  group: AlertGroup;
}

const silencedBadgeClassName =
  "text-[10px] font-mono uppercase tracking-wider bg-primary/10 text-primary border-primary/30";

export function AlertGroupCard({ group }: AlertGroupCardProps) {
  const [open, setOpen] = useState(false);

  const activeCount = group.alerts.filter((a) => a.state === "active").length;
  const silencedCount = group.alerts.filter((a) => isSilenced(a)).length;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger asChild>
        <Button
          variant="ghost"
          className="w-full flex items-center justify-between px-3 py-2 h-auto rounded-md hover:bg-muted/50"
          aria-expanded={open}
          aria-controls={`alert-group-${group.name}`}
        >
          <div className="flex items-center gap-2">
            <ChevronRightIcon
              className={cn(
                "size-4 text-muted-foreground transition-transform duration-200 shrink-0",
                open && "rotate-90"
              )}
              aria-hidden="true"
            />
            <span className="font-medium text-sm tracking-tight">{group.name}</span>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {activeCount > 0 && (
              <Badge variant="destructive" className="text-[10px] font-mono uppercase tracking-wider">
                {activeCount}
              </Badge>
            )}
            {silencedCount > 0 && (
              <Badge
                variant="outline"
                className={silencedBadgeClassName}
              >
                {silencedCount}
              </Badge>
            )}
            {activeCount === 0 && silencedCount === 0 && (
              <Badge variant="secondary" className="text-[10px] font-mono uppercase tracking-wider">
                {group.alerts.length}
              </Badge>
            )}
          </div>
        </Button>
      </CollapsibleTrigger>

      <CollapsibleContent id={`alert-group-${group.name}`}>
        <div className="ml-6 mb-2">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>State</TableHead>
                <TableHead className="hidden sm:table-cell">Receivers</TableHead>
                <TableHead>Started</TableHead>
                <TableHead className="hidden sm:table-cell">
                  Summary
                </TableHead>
                <TableHead className="hidden md:table-cell">
                  Instance
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {group.alerts.map((alert) => {
                const summaryText = (
                  alert.annotations["summary"] ??
                  alert.annotations["description"] ??
                  ""
                ).trim();
                const receiversText = alert.receivers.join(", ").trim();

                return (
                <TableRow key={alert.fingerprint}>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {isSilenced(alert) ? (
                        <Badge
                          variant="outline"
                          className={cn("text-xs", silencedBadgeClassName)}
                        >
                          silenced
                        </Badge>
                      ) : (
                        <Badge
                          variant={
                            alert.state === "active"
                              ? "destructive"
                              : "secondary"
                          }
                          className="text-xs"
                        >
                          {alert.state}
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs hidden sm:table-cell max-w-[10rem]">
                    {receiversText ? (
                      <TruncatedCopyTooltipText
                        text={receiversText}
                        enableCopy={false}
                        triggerClassName="max-w-[10rem]"
                      />
                    ) : (
                      "\u2014"
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {formatAlertTime(alert.startsAt)}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs hidden sm:table-cell max-w-[16rem]">
                    {summaryText ? (
                      <TruncatedCopyTooltipText
                        text={summaryText}
                        copyAriaLabel="Click to copy summary"
                        triggerClassName="max-w-[16rem]"
                      />
                    ) : (
                      "\u2014"
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs hidden md:table-cell max-w-[12rem] truncate">
                    {alert.labels["instance"] ?? "\u2014"}
                  </TableCell>
                </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
