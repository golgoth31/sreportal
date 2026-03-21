import { Skeleton } from "@/components/ui/skeleton";

import type { ReleaseEntry } from "../domain/release.types";
import { ReleaseCard } from "./ReleaseCard";

interface ReleaseListProps {
  entries: readonly ReleaseEntry[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

export function ReleaseList({
  entries,
  isLoading,
  hasFilters,
  onClearFilters,
}: ReleaseListProps) {
  if (isLoading) {
    return (
      <div className="space-y-1.5">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-8 w-full rounded-md" />
        ))}
      </div>
    );
  }

  if (entries.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        {hasFilters ? (
          <>
            <p>No releases match your search.</p>
            <button
              onClick={onClearFilters}
              className="text-primary hover:underline text-sm mt-1"
            >
              Clear filters
            </button>
          </>
        ) : (
          <p>No releases for this day.</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {entries.map((entry, i) => (
        <ReleaseCard key={`${entry.version}-${entry.date}-${i}`} entry={entry} />
      ))}
    </div>
  );
}
