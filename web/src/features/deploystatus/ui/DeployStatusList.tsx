import { SearchXIcon } from "lucide-react";

import { Skeleton } from "@/components/ui/skeleton";
import type { DeployStatusEntry } from "../domain/deploystatus.types";
import { DeployStatusCard } from "./DeployStatusCard";

interface DeployStatusListProps {
  entries: DeployStatusEntry[];
  isLoading: boolean;
}

export function DeployStatusList({ entries, isLoading }: DeployStatusListProps) {
  if (isLoading) {
    return (
      <div className="space-y-3" aria-label="Loading deploy status entries">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-24 w-full rounded-lg" />
        ))}
      </div>
    );
  }
  if (entries.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <SearchXIcon className="size-10 text-muted-foreground" />
        <p className="text-muted-foreground text-sm">No deploy status entries found.</p>
      </div>
    );
  }
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {entries.map((entry) => (
        <DeployStatusCard key={entry.key} entry={entry} />
      ))}
    </div>
  );
}
