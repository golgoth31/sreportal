import type { LucideIcon } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface StatCardProps {
  readonly title: string;
  readonly value: number;
  readonly icon: LucideIcon;
  readonly description?: string;
}

export function StatCard({ title, value, icon: Icon, description }: StatCardProps) {
  return (
    <Card size="sm" className="border-border/70 bg-card/60 backdrop-blur-sm transition-colors hover:border-primary/30">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-[10px] font-mono uppercase tracking-[0.16em] text-muted-foreground">
            {title}
          </CardTitle>
          <Icon className="h-4 w-4 text-primary/70" aria-hidden="true" />
        </div>
      </CardHeader>
      <CardContent>
        <p
          className="font-display text-4xl leading-none tracking-tight"
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
