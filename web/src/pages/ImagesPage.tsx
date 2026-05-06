import {
  ArrowUpCircleIcon,
  CheckIcon,
  LayersIcon,
  PackagePlusIcon,
  WandSparklesIcon,
} from "lucide-react";
import { useMemo } from "react";
import { useParams } from "react-router";

import { ErrorAlert } from "@/components/ErrorAlert";
import { FilterBar, type ActiveFilter } from "@/components/FilterBar";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { useImages } from "@/features/image/hooks/useImages";
import { changeTypeBadgeClass, tagTypeBadgeClass, tagTypeBadgeMutedClass } from "@/features/image/ui/image.badge-utils";
import { ImageGroupList } from "@/features/image/ui/ImageGroupList";
import { cn } from "@/lib/utils";

const MUTATED_BADGE_ACTIVE =
  "border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
const MUTATED_BADGE_MUTED =
  "border-amber-300/70 bg-transparent text-amber-700/70 hover:bg-amber-50 hover:text-amber-800 dark:border-amber-800/50 dark:text-amber-400/70 dark:hover:bg-amber-900/20 dark:hover:text-amber-300";
const INJECTED_BADGE_ACTIVE =
  "border-violet-300 bg-violet-100 text-violet-800 dark:border-violet-700 dark:bg-violet-900/30 dark:text-violet-300";
const INJECTED_BADGE_MUTED =
  "border-violet-300/70 bg-transparent text-violet-700/70 hover:bg-violet-50 hover:text-violet-800 dark:border-violet-800/50 dark:text-violet-400/70 dark:hover:bg-violet-900/20 dark:hover:text-violet-300";
const UPGRADE_BADGE_ACTIVE =
  "border-emerald-300 bg-emerald-100 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
const UPGRADE_BADGE_MUTED =
  "border-emerald-300/70 bg-transparent text-emerald-700/70 hover:bg-emerald-50 hover:text-emerald-800 dark:border-emerald-800/50 dark:text-emerald-400/70 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-300";

const CHANGE_TYPES = ["none", "mutated", "injected"] as const;

export function ImagesPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const {
    groupedByRegistry,
    isLoading,
    isFetching,
    error,
    search,
    setSearch,
    tagTypeFilter,
    setTagTypeFilter,
    mutatedFilter,
    setMutatedFilter,
    injectedFilter,
    setInjectedFilter,
    namespaceFilter,
    toggleNamespace,
    changeTypeFilter,
    setChangeTypeFilter,
    upgradeFilter,
    setUpgradeFilter,
    groupByHost,
    setGroupByHost,
    webhookCounts,
    upgradeCount,
    namespaces,
    filteredCount,
    totalCount,
    hasFilters,
    clearAllFilters,
    refetch,
  } = useImages(portalName);

  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (search) filters.push({ label: "search", value: search, onRemove: () => setSearch("") });
    if (tagTypeFilter)
      filters.push({ label: "tagType", value: tagTypeFilter, onRemove: () => setTagTypeFilter("") });
    if (mutatedFilter)
      filters.push({ label: "webhook", value: "mutated", onRemove: () => setMutatedFilter(false) });
    if (injectedFilter)
      filters.push({ label: "webhook", value: "injected", onRemove: () => setInjectedFilter(false) });
    if (changeTypeFilter)
      filters.push({
        label: "changeType",
        value: changeTypeFilter,
        onRemove: () => setChangeTypeFilter(""),
      });
    if (upgradeFilter)
      filters.push({ label: "upgrade", value: "available", onRemove: () => setUpgradeFilter(false) });
    for (const ns of namespaceFilter) {
      filters.push({
        label: "ns",
        value: ns,
        onRemove: () => toggleNamespace(ns),
      });
    }
    return filters;
  }, [
    search,
    tagTypeFilter,
    mutatedFilter,
    injectedFilter,
    changeTypeFilter,
    upgradeFilter,
    namespaceFilter,
    setSearch,
    setTagTypeFilter,
    setMutatedFilter,
    setInjectedFilter,
    setChangeTypeFilter,
    setUpgradeFilter,
    toggleNamespace,
  ]);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="font-display text-3xl tracking-tight">
          Image <span className="italic text-primary">inventory</span>
        </h1>
        <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">
          {/* Group by host toggle */}
          <Button
            variant={groupByHost ? "secondary" : "outline"}
            size="sm"
            onClick={() => setGroupByHost(!groupByHost)}
            aria-pressed={groupByHost}
            className="gap-1.5"
          >
            <LayersIcon className="size-3.5" aria-hidden="true" />
            Group by host
            {groupByHost && <CheckIcon className="size-3.5" aria-hidden="true" />}
          </Button>
          <PageRefreshButton onRefresh={() => void refetch()} isFetching={isFetching} />
          {!isLoading && !error && (
            <span className="text-muted-foreground text-sm font-mono">
              {hasFilters ? `${filteredCount} / ${totalCount} images` : `${totalCount} images`}
            </span>
          )}
        </div>
      </div>

      {/* Quick filter badges row */}
      <div className="flex gap-2 flex-wrap items-center">
        {/* Tag type quick-filters */}
        {(["semver", "commit", "digest", "latest", "other"] as const).map((tag) => (
          <Badge
            key={tag}
            variant="outline"
            role="button"
            aria-pressed={tagTypeFilter === tag}
            tabIndex={0}
            className={cn(
              "cursor-pointer transition-colors",
              tagTypeFilter === tag ? tagTypeBadgeClass(tag) : tagTypeBadgeMutedClass(tag),
            )}
            onClick={() => setTagTypeFilter(tagTypeFilter === tag ? "" : tag)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ")
                setTagTypeFilter(tagTypeFilter === tag ? "" : tag);
            }}
          >
            {tag}
          </Badge>
        ))}

        <Separator orientation="vertical" className="mx-1 h-4" />

        {/* Webhook activity filters */}
        <Badge
          variant="outline"
          role="button"
          aria-pressed={mutatedFilter}
          tabIndex={0}
          className={cn(
            "cursor-pointer transition-colors gap-1",
            mutatedFilter ? MUTATED_BADGE_ACTIVE : MUTATED_BADGE_MUTED,
          )}
          onClick={() => setMutatedFilter(!mutatedFilter)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") setMutatedFilter(!mutatedFilter);
          }}
        >
          <WandSparklesIcon className="size-3" aria-hidden="true" />
          mutated
          <span className="ml-0.5 font-mono text-[10px] opacity-70">{webhookCounts.mutated}</span>
        </Badge>
        <Badge
          variant="outline"
          role="button"
          aria-pressed={injectedFilter}
          tabIndex={0}
          className={cn(
            "cursor-pointer transition-colors gap-1",
            injectedFilter ? INJECTED_BADGE_ACTIVE : INJECTED_BADGE_MUTED,
          )}
          onClick={() => setInjectedFilter(!injectedFilter)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") setInjectedFilter(!injectedFilter);
          }}
        >
          <PackagePlusIcon className="size-3" aria-hidden="true" />
          injected
          <span className="ml-0.5 font-mono text-[10px] opacity-70">{webhookCounts.injected}</span>
        </Badge>

        {/* Upgrades shortcut — shown only when there are upgrades */}
        {upgradeCount > 0 && (
          <Badge
            variant="outline"
            role="button"
            aria-pressed={upgradeFilter}
            tabIndex={0}
            className={cn(
              "cursor-pointer transition-colors gap-1",
              upgradeFilter ? UPGRADE_BADGE_ACTIVE : UPGRADE_BADGE_MUTED,
            )}
            onClick={() => setUpgradeFilter(!upgradeFilter)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") setUpgradeFilter(!upgradeFilter);
            }}
          >
            <ArrowUpCircleIcon className="size-3" aria-hidden="true" />
            upgrades
            <span className="ml-0.5 font-mono text-[10px] opacity-70">{upgradeCount}</span>
          </Badge>
        )}

        {/* Change type facet */}
        {CHANGE_TYPES.map((ct) => {
          const cls = changeTypeBadgeClass(ct);
          if (!cls) return null;
          return (
            <Badge
              key={ct}
              variant="outline"
              role="button"
              aria-pressed={changeTypeFilter === ct}
              tabIndex={0}
              className={cn(
                "cursor-pointer transition-colors",
                changeTypeFilter === ct
                  ? cls
                  : "border-gray-200/70 bg-transparent text-gray-600/70 hover:bg-gray-50 dark:border-gray-700/50 dark:text-gray-400/70 dark:hover:bg-gray-800/30",
              )}
              onClick={() => setChangeTypeFilter(changeTypeFilter === ct ? "" : ct)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ")
                  setChangeTypeFilter(changeTypeFilter === ct ? "" : ct);
              }}
            >
              {ct}
            </Badge>
          );
        })}

        {/* Namespace multi-select popover */}
        {namespaces.length > 0 && (
          <Popover>
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                className={cn(
                  "h-6 gap-1 px-2 text-[11px] font-normal",
                  namespaceFilter.length > 0 &&
                    "border-primary/50 bg-primary/5 text-primary",
                )}
                aria-label="Filter by namespace"
              >
                <LayersIcon className="size-3" aria-hidden="true" />
                namespace
                {namespaceFilter.length > 0 && (
                  <span className="ml-0.5 font-mono text-[10px] opacity-70">
                    {namespaceFilter.length}
                  </span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-56 p-1">
              <ul role="listbox" aria-multiselectable="true" aria-label="Namespace filter">
                {namespaces.map((ns) => {
                  const selected = namespaceFilter.includes(ns);
                  return (
                    <li key={ns}>
                      <button
                        type="button"
                        role="option"
                        aria-selected={selected}
                        className={cn(
                          "w-full flex items-center gap-2 rounded px-2 py-1.5 text-sm transition-colors",
                          "hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                          selected && "bg-primary/10 text-primary",
                        )}
                        onClick={() => toggleNamespace(ns)}
                      >
                        <span
                          className={cn(
                            "size-3.5 rounded border flex items-center justify-center shrink-0",
                            selected
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-border",
                          )}
                          aria-hidden="true"
                        >
                          {selected && <CheckIcon className="size-2.5" />}
                        </span>
                        <span className="font-mono text-xs truncate">{ns}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </PopoverContent>
          </Popover>
        )}
      </div>

      {/* Search + active filter chips */}
      <FilterBar
        searchValue={search}
        searchPlaceholder="Search repositories…"
        searchAriaLabel="Search images"
        onSearchChange={setSearch}
        hasFilters={hasFilters}
        onClearFilters={clearAllFilters}
        activeFilters={activeFilters}
      />

      {error && <ErrorAlert title="Failed to load images" error={error} />}

      {!error && (
        <ImageGroupList
          groups={groupedByRegistry}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearAllFilters}
          showGroupStats={groupByHost}
        />
      )}
    </div>
  );
}
