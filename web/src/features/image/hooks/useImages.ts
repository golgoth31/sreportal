import { useCallback, useMemo, useState } from "react";

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
  const [namespaceFilter, setNamespaceFilter] = useState<string[]>([]);
  const [changeTypeFilter, setChangeTypeFilter] = useState("");
  const [upgradeFilter, setUpgradeFilter] = useState(false);
  const [groupByHost, setGroupByHost] = useState(false);

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
        namespaceFilter,
        changeTypeFilter,
        upgradeFilter,
      }),
    [
      images,
      search,
      registryFilter,
      tagTypeFilter,
      mutatedFilter,
      injectedFilter,
      namespaceFilter,
      changeTypeFilter,
      upgradeFilter,
    ],
  );

  const groupedByRegistry = useMemo(() => groupImagesByRegistry(filtered), [filtered]);

  const registries = useMemo(
    () => [...new Set(images.map((img) => img.registry))].sort(),
    [images],
  );

  // All namespaces present across all (unfiltered) images.
  const namespaces = useMemo(() => {
    const ns = new Set<string>();
    for (const img of images) {
      for (const w of img.workloads) {
        if (!w.hidden) ns.add(w.namespace);
      }
    }
    return [...ns].sort();
  }, [images]);

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

  const upgradeCount = useMemo(
    () => images.reduce((n, img) => n + (img.upgradeAvailable ? 1 : 0), 0),
    [images],
  );

  const toggleNamespace = useCallback((ns: string) => {
    setNamespaceFilter((prev) =>
      prev.includes(ns) ? prev.filter((n) => n !== ns) : [...prev, ns],
    );
  }, []);

  const clearAllFilters = useCallback(() => {
    setSearch("");
    setRegistryFilter("");
    setTagTypeFilter("");
    setMutatedFilter(false);
    setInjectedFilter(false);
    setNamespaceFilter([]);
    setChangeTypeFilter("");
    setUpgradeFilter(false);
  }, []);

  const hasFilters =
    search !== "" ||
    registryFilter !== "" ||
    tagTypeFilter !== "" ||
    mutatedFilter ||
    injectedFilter ||
    namespaceFilter.length > 0 ||
    changeTypeFilter !== "" ||
    upgradeFilter;

  return {
    images,
    filtered,
    groupedByRegistry,
    registries,
    namespaces,
    countsByTag,
    webhookCounts,
    upgradeCount,
    groupByHost,
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
    namespaceFilter,
    changeTypeFilter,
    upgradeFilter,
    hasFilters,
    setSearch,
    setRegistryFilter,
    setTagTypeFilter,
    setMutatedFilter,
    setInjectedFilter,
    setNamespaceFilter,
    toggleNamespace,
    setChangeTypeFilter,
    setUpgradeFilter,
    setGroupByHost,
    clearAllFilters,
    refetch,
  };
}
