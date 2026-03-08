import {
  ChevronDownIcon,
  ExternalLinkIcon,
  AlertTriangleIcon,
} from "lucide-react";
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
import { cn } from "@/lib/utils";
import type { AlertmanagerResource } from "../domain/alertmanager.types";
import { formatAlertTime, getAlertName } from "../domain/alertmanager.types";

interface AlertmanagerResourceCardProps {
  resource: AlertmanagerResource;
}

export function AlertmanagerResourceCard({ resource }: AlertmanagerResourceCardProps) {
  const [open, setOpen] = useState(true);
  const alertCount = resource.alerts.length;
  const displayUrl = resource.remoteUrl || resource.localUrl;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <div className="rounded-lg border bg-card shadow-xs overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button
            variant="ghost"
            className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/50"
          >
            <div className="flex items-center gap-3 flex-wrap">
              <AlertTriangleIcon className="size-4 text-muted-foreground shrink-0" />
              <span className="font-semibold text-sm">{resource.name}</span>
              <span className="text-muted-foreground text-xs">
                {resource.namespace}
              </span>
              <span className="text-muted-foreground text-xs">
                {alertCount} {alertCount === 1 ? "alert" : "alerts"}
              </span>
              {resource.ready ? (
                <Badge variant="secondary" className="text-xs">
                  Ready
                </Badge>
              ) : (
                <Badge variant="outline" className="text-xs text-muted-foreground">
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
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Alert</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>Started</TableHead>
                    <TableHead className="hidden sm:table-cell">Summary</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {resource.alerts.map((alert) => (
                    <TableRow key={alert.fingerprint}>
                      <TableCell className="font-medium">
                        {getAlertName(alert) || alert.fingerprint.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            alert.state === "active" ? "destructive" : "secondary"
                          }
                          className="text-xs"
                        >
                          {alert.state}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground text-xs">
                        {formatAlertTime(alert.startsAt)}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-xs hidden sm:table-cell max-w-[12rem] truncate">
                        {alert.annotations["summary"] ??
                          alert.annotations["description"] ??
                          "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
