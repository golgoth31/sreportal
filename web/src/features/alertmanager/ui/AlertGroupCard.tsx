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
import { cn } from "@/lib/utils";
import type { AlertGroup } from "../domain/alertmanager.types";
import { formatAlertTime } from "../domain/alertmanager.types";

interface AlertGroupCardProps {
  group: AlertGroup;
}

export function AlertGroupCard({ group }: AlertGroupCardProps) {
  const [open, setOpen] = useState(false);
  const count = group.alerts.length;

  const hasActive = group.alerts.some((a) => a.state === "active");

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
            <span className="font-medium text-sm">{group.name}</span>
          </div>
          <Badge
            variant={hasActive ? "destructive" : "secondary"}
            className="text-xs"
          >
            {count}
          </Badge>
        </Button>
      </CollapsibleTrigger>

      <CollapsibleContent id={`alert-group-${group.name}`}>
        <div className="ml-6 mb-2">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>State</TableHead>
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
              {group.alerts.map((alert) => (
                <TableRow key={alert.fingerprint}>
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
                  <TableCell className="text-muted-foreground text-xs hidden sm:table-cell max-w-[16rem] truncate">
                    {alert.annotations["summary"] ??
                      alert.annotations["description"] ??
                      "\u2014"}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs hidden md:table-cell max-w-[12rem] truncate">
                    {alert.labels["instance"] ?? "\u2014"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
