import { BarChart3Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { MetricFamily } from "../domain/metrics.types";
import { MetricFamilyCard } from "./MetricFamilyCard";

interface MetricListProps {
  families: MetricFamily[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

export function MetricList({
  families,
  isLoading,
  hasFilters,
  onClearFilters,
}: MetricListProps) {
  if (isLoading) {
    return (
      <div className="space-y-3" aria-label="Loading metrics">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (families.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <BarChart3Icon className="size-10 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">
          No metrics found.
        </p>
        {hasFilters && (
          <Button variant="outline" size="sm" onClick={onClearFilters}>
            Clear filters
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {families.map((family) => (
        <MetricFamilyCard key={family.name} family={family} />
      ))}
    </div>
  );
}
