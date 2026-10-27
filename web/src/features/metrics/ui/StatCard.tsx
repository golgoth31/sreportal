import type { LucideIcon } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface StatCardProps {
  readonly title: string;
  readonly value: number;
  readonly icon: LucideIcon;
  readonly description?: string;
  readonly density?: "default" | "compact";
}

export function StatCard({
  title,
  value,
  icon: Icon,
  description,
  density = "default",
}: StatCardProps) {
  const compact = density === "compact";
  return (
    <Card
      size="sm"
      className={cn(
        "transition-shadow hover:shadow-[var(--elevation-2)]",
        compact && "data-[size=sm]:gap-1 data-[size=sm]:py-1.5",
      )}
    >
      <CardHeader className={compact ? "group-data-[size=sm]/card:px-2.5" : undefined}>
        <div className="flex items-center justify-between">
          <CardTitle className="text-[10px] font-mono uppercase tracking-[0.16em] text-muted-foreground">
            {title}
          </CardTitle>
          <Icon
            className={cn(
              "text-primary/70",
              compact ? "h-3.5 w-3.5" : "h-4 w-4",
            )}
            aria-hidden="true"
          />
        </div>
      </CardHeader>
      <CardContent className={compact ? "group-data-[size=sm]/card:px-2.5" : undefined}>
        <p
          className={cn(
            "font-display leading-none tracking-tight",
            compact ? "text-2xl" : "text-4xl",
          )}
          aria-label={`${title}: ${value}`}
        >
          {value.toLocaleString()}
        </p>
        {description && (
          <p className="text-xs text-muted-foreground mt-2">{description}</p>
        )}
      </CardContent>
    </Card>
  );
}
