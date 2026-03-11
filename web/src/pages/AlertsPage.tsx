import { useMemo } from "react";
import { useParams } from "react-router";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ErrorAlert } from "@/components/ErrorAlert";
import { FilterBar, type ActiveFilter } from "@/components/FilterBar";
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

  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (search) {
      filters.push({
        label: "search",
        value: search,
        onRemove: () => setSearch(""),
      });
    }
    if (stateFilter) {
      filters.push({
        label: "state",
        value: stateFilter,
        onRemove: () => setStateFilter(""),
      });
    }
    return filters;
  }, [search, stateFilter, setSearch, setStateFilter]);

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

      {/* Filters */}
      <FilterBar
        searchValue={search}
        searchPlaceholder="Search by label or annotation…"
        searchAriaLabel="Search alerts"
        onSearchChange={setSearch}
        hasFilters={hasFilters}
        onClearFilters={clearFilters}
        activeFilters={activeFilters}
      >
        <Select
          value={stateFilter || ALL_STATES_VALUE}
          onValueChange={(v) =>
            setStateFilter(v === ALL_STATES_VALUE ? "" : v)
          }
        >
          <SelectTrigger className="w-40" aria-label="Filter by state">
            <SelectValue placeholder="State" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_STATES_VALUE}>All states</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="suppressed">Suppressed</SelectItem>
            <SelectItem value="silenced">Silenced</SelectItem>
            <SelectItem value="unprocessed">Unprocessed</SelectItem>
          </SelectContent>
        </Select>
      </FilterBar>

      {/* Error state */}
      {error && <ErrorAlert title="Failed to load alerts" error={error} />}

      {/* Alert list */}
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
