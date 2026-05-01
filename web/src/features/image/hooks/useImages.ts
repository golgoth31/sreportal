import { useMemo, useState } from "react";

import {
  annotateImages,
  filterImages,
  groupImagesByRegistry,
  hasVisibleWorkloads,
} from "../domain/image.types";
import { useImageQuery } from "./useImageQuery";

export function useImages(portal: string) {
  const [search, setSearch] = useState("");
  const [registryFilter, setRegistryFilter] = useState("");
  const [tagTypeFilter, setTagTypeFilter] = useState("");
  const { images: rawImages, isLoading, isFetching, error, refetch } = useImageQuery(portal);

  const images = useMemo(
    () => annotateImages(rawImages).filter(hasVisibleWorkloads),
    [rawImages],
  );

  const filtered = useMemo(
    () => filterImages(images, { search, registryFilter, tagTypeFilter }),
    [images, search, registryFilter, tagTypeFilter],
  );
  const groupedByRegistry = useMemo(() => groupImagesByRegistry(filtered), [filtered]);
  const registries = useMemo(
    () => [...new Set(images.map((img) => img.registry))].sort(),
    [images],
  );

  const countsByTag = useMemo(() => {
    return filtered.reduce<Record<string, number>>(
      (acc, img) => {
        acc[img.tagType] = (acc[img.tagType] ?? 0) + 1;
        return acc;
      },
      { semver: 0, commit: 0, digest: 0, latest: 0 },
    );
  }, [filtered]);

  return {
    images,
    filtered,
    groupedByRegistry,
    registries,
    countsByTag,
    totalCount: images.length,
    filteredCount: filtered.length,
    isLoading,
    isFetching,
    error,
    search,
    registryFilter,
    tagTypeFilter,
    setSearch,
    setRegistryFilter,
    setTagTypeFilter,
    refetch,
  };
}
