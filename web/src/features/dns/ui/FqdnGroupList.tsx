import { SearchXIcon } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody } from "@/components/ui/table";
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
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(
    () => new Set(),
  );

  if (isLoading) {
    return (
      <div
        className="flex flex-col gap-[var(--density-gap)]"
        aria-label="Loading FQDN groups"
      >
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-[var(--row-height)] w-full rounded-md" />
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
    <div className="surface overflow-hidden">
      <Table className="table-fixed">
        <colgroup>
          <col className="w-8" />
          <col />
          <col className="w-[4.5rem]" />
          <col />
          <col className="w-[7.5rem]" />
          <col className="w-10" />
        </colgroup>
        <TableBody>
          {groups.map((group) => {
            const open = !collapsed.has(group.name);
            return (
              <FqdnGroupCard
                key={group.name}
                group={group}
                open={open}
                onOpenChange={(next) => {
                  setCollapsed((prev) => {
                    const nextSet = new Set(prev);
                    if (next) nextSet.delete(group.name);
                    else nextSet.add(group.name);
                    return nextSet;
                  });
                }}
              />
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
