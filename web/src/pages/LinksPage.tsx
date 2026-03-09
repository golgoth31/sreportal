import { ExternalLinkIcon } from "lucide-react";
import { useMemo } from "react";
import { useParams } from "react-router";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ErrorAlert } from "@/components/ErrorAlert";
import { FilterBar, type ActiveFilter } from "@/components/FilterBar";
import { useDns } from "@/features/dns/hooks/useDns";
import { FqdnGroupList } from "@/features/dns/ui/FqdnGroupList";
import { usePortals } from "@/features/portal/hooks/usePortals";

const ALL_GROUPS_VALUE = "__all__";

export function LinksPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const {
    groupedByGroup,
    groups,
    totalCount,
    filteredCount,
    isLoading,
    error,
    searchTerm,
    groupFilter,
    setSearchTerm,
    setGroupFilter,
    clearFilters,
  } = useDns(portalName);

  const { portals } = usePortals();
  const currentPortal = portals.find(
    (p) => (p.subPath || p.name) === portalName
  );
  const hasFilters = searchTerm !== "" || groupFilter !== "";

  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (searchTerm) {
      filters.push({
        label: "search",
        value: searchTerm,
        onRemove: () => setSearchTerm(""),
      });
    }
    if (groupFilter) {
      filters.push({
        label: "group",
        value: groupFilter,
        onRemove: () => setGroupFilter(""),
      });
    }
    return filters;
  }, [searchTerm, groupFilter, setSearchTerm, setGroupFilter]);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-semibold tracking-tight">DNS Links</h1>
          {currentPortal?.isRemote && currentPortal.url && (
            <Button variant="outline" size="sm" asChild>
              <a
                href={currentPortal.url}
                target="_blank"
                rel="noopener noreferrer"
              >
                Remote portal
                <ExternalLinkIcon className="size-3" />
              </a>
            </Button>
          )}
        </div>

        {/* Stats */}
        {!isLoading && !error && (
          <span className="text-muted-foreground text-sm ml-auto">
            {hasFilters
              ? `${filteredCount} of ${totalCount} FQDNs`
              : `${totalCount} FQDNs`}
          </span>
        )}
      </div>

      {/* Filters */}
      <FilterBar
        searchValue={searchTerm}
        searchPlaceholder="Search FQDNs…"
        searchAriaLabel="Search FQDNs"
        onSearchChange={setSearchTerm}
        hasFilters={hasFilters}
        onClearFilters={clearFilters}
        activeFilters={activeFilters}
      >
        <Select
          value={groupFilter || ALL_GROUPS_VALUE}
          onValueChange={(v) =>
            setGroupFilter(v === ALL_GROUPS_VALUE ? "" : v)
          }
        >
          <SelectTrigger className="w-48" aria-label="Filter by group">
            <SelectValue placeholder="All groups" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_GROUPS_VALUE}>All groups</SelectItem>
            {groups.map((g) => (
              <SelectItem key={g} value={g}>
                {g}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FilterBar>

      {/* Error state */}
      {error && <ErrorAlert title="Failed to load FQDNs" error={error} />}

      {/* FQDN groups */}
      {!error && (
        <FqdnGroupList
          groups={groupedByGroup}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearFilters}
        />
      )}
    </div>
  );
}
