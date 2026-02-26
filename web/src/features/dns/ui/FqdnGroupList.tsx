import { SearchXIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { FqdnGroup } from "../domain/dns.types";
import { FqdnGroupCard } from "./FqdnGroupCard";

interface FqdnGroupListProps {
  groups: FqdnGroup[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

export function FqdnGroupList({
  groups,
  isLoading,
  hasFilters,
  onClearFilters,
}: FqdnGroupListProps) {
  if (isLoading) {
    return (
      <div className="space-y-3" aria-label="Loading FQDN groups">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (groups.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <SearchXIcon className="size-10 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">No FQDNs found.</p>
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
      {groups.map((group) => (
        <FqdnGroupCard key={group.name} group={group} />
      ))}
    </div>
  );
}
