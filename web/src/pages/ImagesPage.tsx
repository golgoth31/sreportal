import { useMemo } from "react";
import { PackagePlusIcon, WandSparklesIcon } from "lucide-react";
import { useParams } from "react-router";

import { ErrorAlert } from "@/components/ErrorAlert";
import { FilterBar, type ActiveFilter } from "@/components/FilterBar";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { useImages } from "@/features/image/hooks/useImages";
import { ImageGroupList } from "@/features/image/ui/ImageGroupList";
import { tagTypeBadgeClass, tagTypeBadgeMutedClass } from "@/features/image/ui/ImageCard";
import { cn } from "@/lib/utils";

const MUTATED_BADGE_ACTIVE =
  "border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
const MUTATED_BADGE_MUTED =
  "border-amber-300/70 bg-transparent text-amber-700/70 hover:bg-amber-50 hover:text-amber-800 dark:border-amber-800/50 dark:text-amber-400/70 dark:hover:bg-amber-900/20 dark:hover:text-amber-300";
const INJECTED_BADGE_ACTIVE =
  "border-violet-300 bg-violet-100 text-violet-800 dark:border-violet-700 dark:bg-violet-900/30 dark:text-violet-300";
const INJECTED_BADGE_MUTED =
  "border-violet-300/70 bg-transparent text-violet-700/70 hover:bg-violet-50 hover:text-violet-800 dark:border-violet-800/50 dark:text-violet-400/70 dark:hover:bg-violet-900/20 dark:hover:text-violet-300";

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
    webhookCounts,
    filteredCount,
    totalCount,
    refetch,
  } = useImages(portalName);

  const hasFilters =
    search !== "" || tagTypeFilter !== "" || mutatedFilter || injectedFilter;
  const clearAllFilters = () => {
    setSearch("");
    setTagTypeFilter("");
    setMutatedFilter(false);
    setInjectedFilter(false);
  };
  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (search) filters.push({ label: "search", value: search, onRemove: () => setSearch("") });
    if (tagTypeFilter) filters.push({ label: "tagType", value: tagTypeFilter, onRemove: () => setTagTypeFilter("") });
    if (mutatedFilter) filters.push({ label: "webhook", value: "mutated", onRemove: () => setMutatedFilter(false) });
    if (injectedFilter) filters.push({ label: "webhook", value: "injected", onRemove: () => setInjectedFilter(false) });
    return filters;
  }, [
    search,
    tagTypeFilter,
    mutatedFilter,
    injectedFilter,
    setSearch,
    setTagTypeFilter,
    setMutatedFilter,
    setInjectedFilter,
  ]);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="font-display text-3xl tracking-tight">
          Image <span className="italic text-primary">inventory</span>
        </h1>
        <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">
          <PageRefreshButton onRefresh={() => void refetch()} isFetching={isFetching} />
          {!isLoading && !error && (
            <span className="text-muted-foreground text-sm font-mono">
              {hasFilters ? `${filteredCount} / ${totalCount} images` : `${totalCount} images`}
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-2 flex-wrap items-center">
        {(["semver", "commit", "digest", "latest", "other"] as const).map((tag) => (
          <Badge
            key={tag}
            variant="outline"
            className={cn(
              "cursor-pointer transition-colors",
              tagTypeFilter === tag
                ? tagTypeBadgeClass(tag)
                : tagTypeBadgeMutedClass(tag),
            )}
            onClick={() => setTagTypeFilter(tagTypeFilter === tag ? "" : tag)}
          >
            {tag}
          </Badge>
        ))}
        <span className="mx-1 h-4 w-px bg-border" aria-hidden="true" />
        <Badge
          variant="outline"
          aria-pressed={mutatedFilter}
          className={cn(
            "cursor-pointer transition-colors gap-1",
            mutatedFilter ? MUTATED_BADGE_ACTIVE : MUTATED_BADGE_MUTED,
          )}
          onClick={() => setMutatedFilter(!mutatedFilter)}
        >
          <WandSparklesIcon className="size-3" />
          mutated
          <span className="ml-0.5 font-mono text-[10px] opacity-70">{webhookCounts.mutated}</span>
        </Badge>
        <Badge
          variant="outline"
          aria-pressed={injectedFilter}
          className={cn(
            "cursor-pointer transition-colors gap-1",
            injectedFilter ? INJECTED_BADGE_ACTIVE : INJECTED_BADGE_MUTED,
          )}
          onClick={() => setInjectedFilter(!injectedFilter)}
        >
          <PackagePlusIcon className="size-3" />
          injected
          <span className="ml-0.5 font-mono text-[10px] opacity-70">{webhookCounts.injected}</span>
        </Badge>
      </div>

      <FilterBar
        searchValue={search}
        searchPlaceholder="Search repositories…"
        searchAriaLabel="Search images"
        onSearchChange={setSearch}
        hasFilters={hasFilters}
        onClearFilters={clearAllFilters}
        activeFilters={activeFilters}
      >
        <Input
          value={tagTypeFilter}
          onChange={(e) => setTagTypeFilter(e.target.value)}
          className="w-48"
          placeholder="Tag type"
          aria-label="Filter by tag type"
        />
      </FilterBar>

      {error && <ErrorAlert title="Failed to load images" error={error} />}

      {!error && (
        <ImageGroupList
          groups={groupedByRegistry}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearAllFilters}
        />
      )}
    </div>
  );
}
