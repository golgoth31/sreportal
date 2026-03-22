import { ArrowDownIcon, ArrowUpIcon, ExternalLinkIcon } from "lucide-react";
import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import type { ReleaseEntry } from "../domain/release.types";
import { formatEntryTime } from "../domain/release.types";

type SortOrder = "desc" | "asc";

interface ReleaseListProps {
  entries: readonly ReleaseEntry[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
  timeZone: string;
}

export function ReleaseList({
  entries,
  isLoading,
  hasFilters,
  onClearFilters,
  timeZone,
}: ReleaseListProps) {
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");

  const sortedEntries = useMemo(() => {
    if (sortOrder === "desc") return entries;
    return [...entries].reverse();
  }, [entries, sortOrder]);

  const toggleSort = () =>
    setSortOrder((prev) => (prev === "desc" ? "asc" : "desc"));

  const SortIcon = sortOrder === "desc" ? ArrowDownIcon : ArrowUpIcon;

  if (isLoading) {
    return <ReleaseTableSkeleton />;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-16">
            <button
              onClick={toggleSort}
              className="inline-flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
              aria-label={`Sort by time ${sortOrder === "desc" ? "ascending" : "descending"}`}
            >
              Time
              <SortIcon className="size-3.5" aria-hidden="true" />
            </button>
          </TableHead>
          <TableHead className="w-28">Type</TableHead>
          <TableHead className="w-32">Version</TableHead>
          <TableHead className="w-28">Origin</TableHead>
          <TableHead>Message</TableHead>
          <TableHead className="w-24">Author</TableHead>
          <TableHead className="w-10 sr-only">Link</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sortedEntries.length === 0 ? (
          <TableRow>
            <TableCell
              colSpan={7}
              className="text-center text-muted-foreground h-24"
            >
              {hasFilters ? (
                <>
                  No releases match your search.{" "}
                  <button
                    onClick={onClearFilters}
                    className="text-primary hover:underline"
                  >
                    Clear filters
                  </button>
                </>
              ) : (
                "No releases for this day."
              )}
            </TableCell>
          </TableRow>
        ) : (
          sortedEntries.map((entry, i) => (
            <TableRow key={`${entry.version}-${entry.date}-${i}`}>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {formatEntryTime(entry.date, timeZone)}
              </TableCell>
              <TableCell>
                <Badge variant="outline" className="text-[11px] px-1.5 py-0">
                  {entry.type}
                </Badge>
              </TableCell>
              <TableCell className="font-medium">{entry.version}</TableCell>
              <TableCell className="text-muted-foreground text-xs">
                {entry.origin}
              </TableCell>
              <TableCell className="text-muted-foreground text-xs truncate max-w-xs">
                {entry.message}
              </TableCell>
              <TableCell className="text-xs">{entry.author}</TableCell>
              <TableCell>
                {entry.link && (
                  <a
                    href={entry.link}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center text-primary hover:underline"
                    aria-label={`Link for ${entry.version}`}
                  >
                    <ExternalLinkIcon className="size-3.5" aria-hidden="true" />
                  </a>
                )}
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}

function ReleaseTableSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full rounded-md" />
      ))}
    </div>
  );
}
