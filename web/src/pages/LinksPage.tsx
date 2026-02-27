import { AlertCircleIcon, ExternalLinkIcon, XIcon } from "lucide-react";
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

      {/* Controls */}
      <div className="flex flex-wrap gap-3 items-end">
        <div className="flex-1 min-w-48">
          <Input
            placeholder="Search FQDNs…"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            aria-label="Search FQDNs"
          />
        </div>

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

        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <XIcon className="size-4" />
            Clear
          </Button>
        )}
      </div>

      {/* Active filters */}
      {hasFilters && (
        <div className="flex flex-wrap gap-1.5 items-center">
          <span className="text-muted-foreground text-xs">Filters:</span>
          {searchTerm && (
            <Badge variant="secondary" className="text-xs gap-1">
              search: {searchTerm}
              <button
                onClick={() => setSearchTerm("")}
                aria-label="Remove search filter"
              >
                <XIcon className="size-3" />
              </button>
            </Badge>
          )}
          {groupFilter && (
            <Badge variant="secondary" className="text-xs gap-1">
              group: {groupFilter}
              <button
                onClick={() => setGroupFilter("")}
                aria-label="Remove group filter"
              >
                <XIcon className="size-3" />
              </button>
            </Badge>
          )}
        </div>
      )}

      {/* Error state */}
      {error && (
        <div
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 text-destructive"
        >
          <AlertCircleIcon className="size-5 shrink-0" />
          <div>
            <p className="font-medium text-sm">Failed to load FQDNs</p>
            <p className="text-xs mt-0.5 opacity-80">
              {error instanceof Error ? error.message : String(error)}
            </p>
          </div>
        </div>
      )}

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
