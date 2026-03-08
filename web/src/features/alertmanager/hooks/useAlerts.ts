import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";

import { listAlerts } from "../infrastructure/alertmanagerApi";
import type { AlertmanagerResource } from "../domain/alertmanager.types";

export interface UseAlertsParams {
  portal: string;
}

export function useAlerts({ portal }: UseAlertsParams) {
  const [search, setSearch] = useState("");
  const [stateFilter, setStateFilter] = useState("");

  const query = useQuery({
    queryKey: ["alerts", portal, search, stateFilter],
    queryFn: () =>
      listAlerts({
        portal: portal || undefined,
        search: search || undefined,
        state: stateFilter || undefined,
      }),
    staleTime: 10_000,
    refetchInterval: 30_000,
  });

  const resources: AlertmanagerResource[] = query.data ?? [];
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
