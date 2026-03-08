import { AlertCircleIcon, XIcon } from "lucide-react";
import { useParams } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { AlertList } from "@/features/alertmanager/ui/AlertList";
import { useAlerts } from "@/features/alertmanager/hooks/useAlerts";

const ALL_STATES_VALUE = "__all__";

export function AlertsPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const {
    resources,
    totalAlerts,
    isLoading,
    error,
    search,
    setSearch,
    stateFilter,
    setStateFilter,
    clearFilters,
    hasFilters,
  } = useAlerts({ portal: portalName });

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="text-xl font-semibold tracking-tight">Alerts</h1>
        {!isLoading && !error && (
          <span className="text-muted-foreground text-sm ml-auto">
            {hasFilters
              ? `${resources.length} resource(s), ${totalAlerts} alert(s)`
              : `${resources.length} Alertmanager(s), ${totalAlerts} alert(s)`}
          </span>
        )}
      </div>

      <div className="flex flex-wrap gap-3 items-end">
        <div className="flex-1 min-w-48">
          <Input
            placeholder="Search by label or annotation…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search alerts"
          />
        </div>
        <Select
          value={stateFilter || ALL_STATES_VALUE}
          onValueChange={(v) => setStateFilter(v === ALL_STATES_VALUE ? "" : v)}
        >
          <SelectTrigger className="w-40" aria-label="Filter by state">
            <SelectValue placeholder="State" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_STATES_VALUE}>All states</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="suppressed">Suppressed</SelectItem>
            <SelectItem value="unprocessed">Unprocessed</SelectItem>
          </SelectContent>
        </Select>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <XIcon className="size-4" />
            Clear
          </Button>
        )}
      </div>

      {hasFilters && (
        <div className="flex flex-wrap gap-1.5 items-center">
          <span className="text-muted-foreground text-xs">Filters:</span>
          {search && (
            <Badge variant="secondary" className="text-xs gap-1">
              search: {search}
              <button
                onClick={() => setSearch("")}
                aria-label="Remove search filter"
              >
                <XIcon className="size-3" />
              </button>
            </Badge>
          )}
          {stateFilter && (
            <Badge variant="secondary" className="text-xs gap-1">
              state: {stateFilter}
              <button
                onClick={() => setStateFilter("")}
                aria-label="Remove state filter"
              >
                <XIcon className="size-3" />
              </button>
            </Badge>
          )}
        </div>
      )}

      {error && (
        <div
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 text-destructive"
        >
          <AlertCircleIcon className="size-5 shrink-0" />
          <div>
            <p className="font-medium text-sm">Failed to load alerts</p>
            <p className="text-xs mt-0.5 opacity-80">
              {error instanceof Error ? error.message : String(error)}
            </p>
          </div>
        </div>
      )}

      {!error && (
        <AlertList
          resources={resources}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearFilters}
        />
      )}
    </div>
  );
}
