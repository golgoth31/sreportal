import type { LucideIcon } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface StatCardProps {
  readonly title: string;
  readonly value: number;
  readonly icon: LucideIcon;
  readonly description?: string;
}

export function StatCard({
  title,
  value,
  icon: Icon,
  description,
}: StatCardProps) {
  return (
    <Card
      size="sm"
      className="transition-shadow hover:shadow-[var(--elevation-2)] data-[size=sm]:gap-1 data-[size=sm]:py-1.5"
    >
      <CardHeader className="group-data-[size=sm]/card:px-2.5">
        <div className="flex items-center justify-between">
          <CardTitle className="text-[10px] font-mono uppercase tracking-[0.16em] text-muted-foreground">
            {title}
          </CardTitle>
          <Icon className="h-3.5 w-3.5 text-primary/70" aria-hidden="true" />
        </div>
      </CardHeader>
      <CardContent className="group-data-[size=sm]/card:px-2.5">
        <p
          className="font-display text-2xl leading-none tracking-tight"
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
