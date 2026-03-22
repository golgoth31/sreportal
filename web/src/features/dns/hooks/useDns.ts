import { useCallback, useMemo, useState } from "react";

import {
  extractGroupNames,
  filterFqdns,
  groupFqdnsByGroup,
} from "../domain/dns.types";
import { useDnsQuery } from "./useDnsQuery";

export function useDns(portal: string) {
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("");

  const { fqdns, isLoading, isFetching, error, refetch } =
    useDnsQuery(portal);

  const filtered = useMemo(
    () => filterFqdns(fqdns, searchTerm, groupFilter),
    [fqdns, searchTerm, groupFilter]
  );

  const groupedByGroup = useMemo(
    () => groupFqdnsByGroup(filtered, groupFilter),
    [filtered, groupFilter]
  );

  const groups = useMemo(() => extractGroupNames(fqdns), [fqdns]);

  const clearFilters = useCallback(() => {
    setSearchTerm("");
    setGroupFilter("");
  }, []);

  return {
    fqdns,
    filtered,
    groupedByGroup,
    groups,
    totalCount: fqdns.length,
    filteredCount: filtered.length,
    isLoading,
    isFetching,
    error,
    searchTerm,
    groupFilter,
    setSearchTerm,
    setGroupFilter,
    clearFilters,
    refetch,
  };
}
