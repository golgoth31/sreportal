import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";

import { listAlerts } from "../infrastructure/alertmanagerApi";
import {
  type Alert,
  type AlertmanagerResource,
  isSilenced,
} from "../domain/alertmanager.types";

const EMPTY_RESOURCES: AlertmanagerResource[] = [];

export interface UseAlertsParams {
  portal: string;
}

function alertMatchesSearch(alert: Alert, term: string): boolean {
  for (const value of Object.values(alert.labels)) {
    if (value.toLowerCase().includes(term)) return true;
  }
  for (const value of Object.values(alert.annotations)) {
    if (value.toLowerCase().includes(term)) return true;
  }
  return false;
}

export function useAlerts({ portal }: UseAlertsParams) {
  const [search, setSearch] = useState("");
  const [stateFilter, setStateFilter] = useState("");

  const query = useQuery({
    queryKey: ["alerts", portal],
    queryFn: () => listAlerts({ portal: portal || undefined }),
    staleTime: 10_000,
    refetchInterval: 30_000,
  });

  const allResources = query.data ?? EMPTY_RESOURCES;

  // Client-side filtering to avoid per-keystroke API calls
  const resources = useMemo(() => {
    const lowerSearch = search.toLowerCase();

    return allResources
      .map((resource) => {
        let filtered = resource.alerts;

        if (stateFilter) {
          if (stateFilter === "silenced") {
            filtered = filtered.filter((a) => isSilenced(a));
          } else {
            filtered = filtered.filter((a) => a.state === stateFilter);
          }
        }

        if (lowerSearch) {
          filtered = filtered.filter((a) =>
            alertMatchesSearch(a, lowerSearch)
          );
        }

        if (filtered.length === resource.alerts.length) return resource;
        return { ...resource, alerts: filtered };
      })
      .filter((r) => r.alerts.length > 0 || (!search && !stateFilter));
  }, [allResources, search, stateFilter]);

  const totalAlerts = useMemo(
    () => resources.reduce((sum, r) => sum + r.alerts.length, 0),
    [resources]
  );

  const clearFilters = useCallback(() => {
    setSearch("");
    setStateFilter("");
  }, []);

  const hasFilters = search !== "" || stateFilter !== "";

  return {
    resources,
    totalAlerts,
    isLoading: query.isLoading,
    error: query.error,
    search,
    setSearch,
    stateFilter,
    setStateFilter,
    clearFilters,
    hasFilters,
    refetch: query.refetch,
  };
}
