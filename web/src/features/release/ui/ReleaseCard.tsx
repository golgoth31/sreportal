import { ExternalLinkIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";

import type { ReleaseEntry } from "../domain/release.types";
import { formatEntryTime } from "../domain/release.types";

interface ReleaseCardProps {
  entry: ReleaseEntry;
}

export function ReleaseCard({ entry }: ReleaseCardProps) {
  return (
    <div className="flex items-center gap-3 rounded-md border bg-card px-3 py-1.5 text-sm">
      <span className="text-muted-foreground text-xs font-mono shrink-0 w-11 text-right">
        {formatEntryTime(entry.date)}
      </span>
      <Badge variant="outline" className="shrink-0 text-[11px] px-1.5 py-0">
        {entry.type}
      </Badge>
      <span className="font-medium shrink-0">{entry.version}</span>
      <span className="text-muted-foreground text-xs shrink-0">{entry.origin}</span>
      {entry.message && (
        <span className="text-muted-foreground text-xs truncate">{entry.message}</span>
      )}
      <div className="ml-auto flex items-center gap-2 shrink-0 text-xs text-muted-foreground">
        {entry.author && <span>{entry.author}</span>}
        {entry.link && (
          <a
            href={entry.link}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-0.5 text-primary hover:underline"
          >
            <ExternalLinkIcon className="size-3" aria-hidden="true" />
          </a>
        )}
      </div>
    </div>
  );
}
