import { SearchXIcon } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody } from "@/components/ui/table";
import type { ImageGroup } from "../domain/image.types";
import { ImageGroupCard } from "./ImageGroupCard";

interface ImageGroupListProps {
  groups: ImageGroup[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

export function ImageGroupList({
  groups,
  isLoading,
  hasFilters,
  onClearFilters,
}: ImageGroupListProps) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(
    () => new Set(),
  );

  if (isLoading) {
    return (
      <div
        className="flex flex-col gap-[var(--density-gap)]"
        aria-label="Loading image groups"
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
        <p className="text-muted-foreground text-sm">No images found.</p>
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
          <col className="w-[8rem]" />
          <col />
          <col className="w-[5.5rem]" />
          <col className="w-[10rem]" />
          <col className="w-16" />
          <col className="w-[7.5rem]" />
        </colgroup>
        <TableBody>
          {groups.map((group) => {
            const open = !collapsed.has(group.registry);
            return (
              <ImageGroupCard
                key={group.registry}
                group={group}
                open={open}
                onOpenChange={(next) => {
                  setCollapsed((prev) => {
                    const nextSet = new Set(prev);
                    if (next) nextSet.delete(group.registry);
                    else nextSet.add(group.registry);
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
