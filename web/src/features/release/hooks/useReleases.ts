import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";

import { filterEntries, sortEntriesByDate } from "../domain/release.types";
import { listReleases } from "../infrastructure/releaseApi";

export function useReleases(portal: string) {
  const [day, setDay] = useState("");
  const [search, setSearch] = useState("");

  const query = useQuery({
    queryKey: ["releases", portal, day],
    queryFn: () => listReleases(day, portal),
    staleTime: 10_000,
  });

  const data = query.data;
  const allEntries = data?.entries ?? [];

  const sorted = useMemo(() => sortEntriesByDate(allEntries), [allEntries]);

  const entries = useMemo(
    () => filterEntries(sorted, search),
    [sorted, search],
  );

  const goToDay = useCallback(
    (target: string) => {
      setDay(target);
      setSearch("");
    },
    [],
  );

  const clearFilters = useCallback(() => {
    setSearch("");
  }, []);

  const hasFilters = search !== "";

  return {
    day: data?.day ?? "",
    entries,
    totalCount: allEntries.length,
    previousDay: data?.previousDay ?? "",
    nextDay: data?.nextDay ?? "",
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
    search,
    setSearch,
    goToDay,
    clearFilters,
    hasFilters,
  };
}
