import { ChevronDownIcon } from "lucide-react";
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
import type { MetricFamily } from "../domain/metrics.types";
import { formatMetricName } from "../domain/metrics.types";

interface MetricFamilyCardProps {
  family: MetricFamily;
}

function typeBadgeVariant(
  type: string,
): "default" | "secondary" | "outline" {
  switch (type) {
    case "GAUGE":
      return "default";
    case "COUNTER":
      return "secondary";
    default:
      return "outline";
  }
}

function formatValue(value: number): string {
  if (Number.isInteger(value)) return value.toLocaleString();
  return value.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  });
}

export function MetricFamilyCard({ family }: MetricFamilyCardProps) {
  const [open, setOpen] = useState(false);
  const displayName = formatMetricName(family.name);
  const labelKeys = extractLabelKeys(family);
  const isHistogram = family.type === "HISTOGRAM";

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <div className="rounded-lg border bg-card shadow-xs overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button
            variant="ghost"
            className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/50"
          >
            <div className="flex items-center gap-3 flex-wrap min-w-0">
              <span className="font-mono font-semibold text-sm truncate">
                {displayName}
              </span>
              <Badge variant={typeBadgeVariant(family.type)} className="text-xs">
                {family.type}
              </Badge>
              <span className="text-muted-foreground text-xs">
                {family.metrics.length}{" "}
                {family.metrics.length === 1 ? "series" : "series"}
              </span>
            </div>
            <ChevronDownIcon
              className={cn(
                "size-4 text-muted-foreground transition-transform duration-200 shrink-0",
                open && "rotate-180",
              )}
            />
          </Button>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="border-t px-4 pb-4 pt-2">
            <p className="text-xs text-muted-foreground mb-3">{family.help}</p>

            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    {labelKeys.map((key) => (
                      <TableHead key={key} className="text-xs">
                        {key}
                      </TableHead>
                    ))}
                    {isHistogram ? (
                      <>
                        <TableHead className="text-xs text-right">
                          Count
                        </TableHead>
                        <TableHead className="text-xs text-right">
                          Sum
                        </TableHead>
                      </>
                    ) : (
                      <TableHead className="text-xs text-right">
                        Value
                      </TableHead>
                    )}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {family.metrics.map((metric, idx) => (
                    <TableRow key={idx}>
                      {labelKeys.map((key) => (
                        <TableCell key={key} className="text-xs font-mono">
                          {metric.labels[key] ?? ""}
                        </TableCell>
                      ))}
                      {isHistogram && metric.histogram ? (
                        <>
                          <TableCell className="text-xs text-right font-mono">
                            {metric.histogram.sampleCount.toLocaleString()}
                          </TableCell>
                          <TableCell className="text-xs text-right font-mono">
                            {formatValue(metric.histogram.sampleSum)}
                          </TableCell>
                        </>
                      ) : (
                        <TableCell className="text-xs text-right font-mono">
                          {formatValue(metric.value)}
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}

function extractLabelKeys(family: MetricFamily): string[] {
  const keys = new Set<string>();
  for (const m of family.metrics) {
    for (const k of Object.keys(m.labels)) {
      keys.add(k);
    }
  }
  return [...keys].sort();
}
