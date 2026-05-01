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
  const [mutatedFilter, setMutatedFilter] = useState(false);
  const [injectedFilter, setInjectedFilter] = useState(false);
  const { images: rawImages, isLoading, isFetching, error, refetch } = useImageQuery(portal);

  const images = useMemo(
    () => annotateImages(rawImages).filter(hasVisibleWorkloads),
    [rawImages],
  );

  const filtered = useMemo(
    () =>
      filterImages(images, {
        search,
        registryFilter,
        tagTypeFilter,
        mutatedFilter,
        injectedFilter,
      }),
    [images, search, registryFilter, tagTypeFilter, mutatedFilter, injectedFilter],
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

  const webhookCounts = useMemo(
    () => ({
      mutated: images.reduce((n, img) => n + (img.hasMutation ? 1 : 0), 0),
      injected: images.reduce((n, img) => n + (img.hasInjection ? 1 : 0), 0),
    }),
    [images],
  );

  return {
    images,
    filtered,
    groupedByRegistry,
    registries,
    countsByTag,
    webhookCounts,
    totalCount: images.length,
    filteredCount: filtered.length,
    isLoading,
    isFetching,
    error,
    search,
    registryFilter,
    tagTypeFilter,
    mutatedFilter,
    injectedFilter,
    setSearch,
    setRegistryFilter,
    setTagTypeFilter,
    setMutatedFilter,
    setInjectedFilter,
    refetch,
  };
}
