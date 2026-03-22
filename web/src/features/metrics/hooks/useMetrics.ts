import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";

import {
  extractSubsystems,
  filterFamilies,
  type MetricFamily,
} from "../domain/metrics.types";
import { listMetrics } from "../infrastructure/metricsApi";

const EMPTY_FAMILIES: MetricFamily[] = [];

export function useMetrics() {
  const [search, setSearch] = useState("");
  const [subsystemFilter, setSubsystemFilter] = useState("");

  const query = useQuery({
    queryKey: ["metrics"],
    queryFn: () => listMetrics(),
    staleTime: 10_000,
  });

  const allFamilies = query.data ?? EMPTY_FAMILIES;

  const families = useMemo(
    () => filterFamilies(allFamilies, search, subsystemFilter),
    [allFamilies, search, subsystemFilter],
  );

  const subsystems = useMemo(
    () => extractSubsystems(allFamilies),
    [allFamilies],
  );

  const totalMetrics = useMemo(
    () => families.reduce((sum, f) => sum + f.metrics.length, 0),
    [families],
  );

  const clearFilters = useCallback(() => {
    setSearch("");
    setSubsystemFilter("");
  }, []);

  const hasFilters = search !== "" || subsystemFilter !== "";

  return {
    allFamilies,
    families,
    subsystems,
    totalMetrics,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
    search,
    setSearch,
    subsystemFilter,
    setSubsystemFilter,
    clearFilters,
    hasFilters,
  };
}
