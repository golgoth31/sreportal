import { ArrowDownIcon, ArrowUpIcon } from "lucide-react";
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { EmojiText } from "@/features/emoji/ui/EmojiText";
import type { ReleaseEntry, ReleaseTypeConfig } from "../domain/release.types";
import { formatEntryTime } from "../domain/release.types";

const FALLBACK_COLOR = "#6b7280";

type SortOrder = "desc" | "asc";

interface ReleaseListProps {
  entries: readonly ReleaseEntry[];
  isLoading: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
  timeZone: string;
  typeColors: readonly ReleaseTypeConfig[];
}

export function ReleaseList({
  entries,
  isLoading,
  hasFilters,
  onClearFilters,
  timeZone,
  typeColors,
}: ReleaseListProps) {
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");

  const colorMap = useMemo(
    () => new Map(typeColors.map((t) => [t.name.toLowerCase(), t.color])),
    [typeColors],
  );

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
        </TableRow>
      </TableHeader>
      <TableBody>
        {sortedEntries.length === 0 ? (
          <TableRow>
            <TableCell
              colSpan={6}
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
            <TableRow key={`${entry.type}-${entry.date}-${i}`}>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {formatEntryTime(entry.date, timeZone)}
              </TableCell>
              <TableCell>
                {(() => {
                  const color = colorMap.get(entry.type.toLowerCase()) ?? FALLBACK_COLOR;
                  return (
                    <Badge
                      variant="outline"
                      className="text-[11px] px-1.5 py-0"
                      style={{
                        borderColor: color,
                        backgroundColor: `${color}1a`,
                        color,
                      }}
                    >
                      {entry.type}
                    </Badge>
                  );
                })()}
              </TableCell>
              <TableCell className="text-xs">
                {entry.link ? (
                  <a
                    href={entry.link}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    {entry.version || "show me"}
                  </a>
                ) : (
                  entry.version
                )}
              </TableCell>
              <TableCell className="text-muted-foreground text-xs">
                {entry.origin}
              </TableCell>
              <TableCell className="text-muted-foreground text-xs max-w-xs">
                <MessageCell message={entry.message} />
              </TableCell>
              <TableCell className="text-xs">{entry.author}</TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}

function MessageCell({ message }: { message: string }) {
  const { copy } = useCopyToClipboard(message);

  if (!message) return null;

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            onClick={() => void copy()}
            className="truncate max-w-xs text-left cursor-pointer hover:text-foreground transition-colors"
            aria-label="Click to copy message"
          >
            <EmojiText text={message} />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-sm whitespace-pre-wrap break-words">
          <EmojiText text={message} />
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
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
