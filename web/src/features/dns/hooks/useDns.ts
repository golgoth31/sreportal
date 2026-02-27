import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";

import { listFqdns } from "../infrastructure/dnsApi";
import type { FqdnGroup } from "../domain/dns.types";

export function useDns(portal: string) {
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("");

  const query = useQuery({
    queryKey: ["fqdns", portal],
    queryFn: () => listFqdns(portal),
    // Poll every 5s to stay in sync with the operator
    refetchInterval: 5_000,
  });

  const fqdns = query.data ?? [];

  const filtered = useMemo(() => {
    const lowerSearch = searchTerm.toLowerCase();
    return fqdns.filter((f) => {
      const matchesSearch =
        !lowerSearch ||
        f.name.toLowerCase().includes(lowerSearch) ||
        f.description.toLowerCase().includes(lowerSearch);

      const matchesGroup =
        !groupFilter || f.groups.includes(groupFilter);

      return matchesSearch && matchesGroup;
    });
  }, [fqdns, searchTerm, groupFilter]);

  // Build groups: each FQDN may belong to multiple groups.
  // When a group filter is active, only the matching group is shown.
  const groupedByGroup = useMemo((): FqdnGroup[] => {
    const groupMap = new Map<string, { source: string; fqdns: typeof filtered }>();

    for (const f of filtered) {
      const targetGroups = groupFilter
        ? f.groups.filter((g) => g === groupFilter)
        : f.groups;

      for (const groupName of targetGroups) {
        if (!groupMap.has(groupName)) {
          groupMap.set(groupName, { source: f.source, fqdns: [] });
        }
        groupMap.get(groupName)!.fqdns.push(f);
      }
    }

    return Array.from(groupMap.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([name, { source, fqdns: groupFqdns }]) => ({
        name,
        source,
        fqdns: [...groupFqdns].sort((a, b) => a.name.localeCompare(b.name)),
      }));
  }, [filtered, groupFilter]);

  const groups = useMemo(
    () => [...new Set(fqdns.flatMap((f) => f.groups))].sort(),
    [fqdns]
  );

  return {
    fqdns,
    filtered,
    groupedByGroup,
    groups,
    totalCount: fqdns.length,
    filteredCount: filtered.length,
    isLoading: query.isLoading,
    error: query.error,
    searchTerm,
    groupFilter,
    setSearchTerm,
    setGroupFilter,
    clearFilters: useCallback(() => {
      setSearchTerm("");
      setGroupFilter("");
    }, []),
    refetch: query.refetch,
  };
}
