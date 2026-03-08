import { AlertTriangleIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { AlertmanagerResource } from "../domain/alertmanager.types";
import { AlertmanagerResourceCard } from "./AlertmanagerResourceCard";

interface AlertListProps {
  resources: AlertmanagerResource[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

export function AlertList({
  resources,
  isLoading,
  hasFilters,
  onClearFilters,
}: AlertListProps) {
  if (isLoading) {
    return (
      <div className="space-y-3" aria-label="Loading alertmanagers">
        {Array.from({ length: 2 }).map((_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (resources.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <AlertTriangleIcon className="size-10 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">
          No Alertmanager resources or alerts found for this portal.
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
      {resources.map((resource) => (
        <AlertmanagerResourceCard key={`${resource.namespace}/${resource.name}`} resource={resource} />
      ))}
    </div>
  );
}
