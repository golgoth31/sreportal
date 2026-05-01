import { useMemo } from "react";
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
    filteredCount,
    totalCount,
    refetch,
  } = useImages(portalName);

  const hasFilters = search !== "" || tagTypeFilter !== "";
  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (search) filters.push({ label: "search", value: search, onRemove: () => setSearch("") });
    if (tagTypeFilter) filters.push({ label: "tagType", value: tagTypeFilter, onRemove: () => setTagTypeFilter("") });
    return filters;
  }, [search, tagTypeFilter, setSearch, setTagTypeFilter]);

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

      <div className="flex gap-2 flex-wrap">
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
      </div>

      <FilterBar
        searchValue={search}
        searchPlaceholder="Search repositories…"
        searchAriaLabel="Search images"
        onSearchChange={setSearch}
        hasFilters={hasFilters}
        onClearFilters={() => {
          setSearch("");
          setTagTypeFilter("");
        }}
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
          onClearFilters={() => {
            setSearch("");
            setTagTypeFilter("");
          }}
        />
      )}
    </div>
  );
}
