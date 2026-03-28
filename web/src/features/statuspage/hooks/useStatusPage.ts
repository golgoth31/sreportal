import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";

import {
  listComponents,
  listMaintenances,
  listIncidents,
} from "../infrastructure/statusApi";
import { computeGlobalStatus } from "../domain/utils";

export interface UseStatusPageParams {
  portal: string;
}

export function useStatusPage({ portal }: UseStatusPageParams) {
  const componentsQuery = useQuery({
    queryKey: ["status-components", portal],
    queryFn: () => listComponents(portal || undefined),
    staleTime: 30_000,
  });

  const maintenancesQuery = useQuery({
    queryKey: ["status-maintenances", portal],
    queryFn: () => listMaintenances(portal || undefined),
    staleTime: 60_000,
  });

  const incidentsQuery = useQuery({
    queryKey: ["status-incidents", portal],
    queryFn: () => listIncidents(portal || undefined),
    staleTime: 30_000,
  });

  const components = componentsQuery.data ?? [];
  const maintenances = maintenancesQuery.data ?? [];
  const incidents = incidentsQuery.data ?? [];

  const globalStatus = useMemo(
    () => computeGlobalStatus(components),
    [components]
  );

  const groupedComponents = useMemo(() => {
    const groups = new Map<string, typeof components>();
    for (const comp of components) {
      const group = comp.group || "Other";
      const existing = groups.get(group) ?? [];
      existing.push(comp);
      groups.set(group, existing);
    }
    return Array.from(groups.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([group, items]) => ({
        group,
        components: items.sort((a, b) =>
          a.displayName.localeCompare(b.displayName)
        ),
      }));
  }, [components]);

  const sortedMaintenances = useMemo(() => {
    const inProgress = maintenances
      .filter((m) => m.phase === "in_progress")
      .sort(
        (a, b) =>
          new Date(a.scheduledEnd).getTime() -
          new Date(b.scheduledEnd).getTime()
      );
    const upcoming = maintenances
      .filter((m) => m.phase === "upcoming")
      .sort(
        (a, b) =>
          new Date(a.scheduledStart).getTime() -
          new Date(b.scheduledStart).getTime()
      );
    const completed = maintenances
      .filter((m) => m.phase === "completed")
      .sort(
        (a, b) =>
          new Date(b.scheduledEnd).getTime() -
          new Date(a.scheduledEnd).getTime()
      )
      .slice(0, 5);
    return [...inProgress, ...upcoming, ...completed];
  }, [maintenances]);

  const isLoading =
    componentsQuery.isLoading ||
    maintenancesQuery.isLoading ||
    incidentsQuery.isLoading;
  const isFetching =
    componentsQuery.isFetching ||
    maintenancesQuery.isFetching ||
    incidentsQuery.isFetching;
  const error: Error | null =
    componentsQuery.error ?? maintenancesQuery.error ?? incidentsQuery.error ?? null;

  const {
    refetch: refetchComponents,
  } = componentsQuery;
  const {
    refetch: refetchMaintenances,
  } = maintenancesQuery;
  const {
    refetch: refetchIncidents,
  } = incidentsQuery;

  const refetch = useCallback(() => {
    refetchComponents();
    refetchMaintenances();
    refetchIncidents();
  }, [refetchComponents, refetchMaintenances, refetchIncidents]);

  const openIncidentCount = useMemo(
    () => incidents.filter((i) => i.currentPhase !== "resolved").length,
    [incidents]
  );

  const ongoingMaintenanceCount = useMemo(
    () => maintenances.filter((m) => m.phase === "in_progress").length,
    [maintenances]
  );

  const dataUpdatedAt = Math.max(
    componentsQuery.dataUpdatedAt,
    maintenancesQuery.dataUpdatedAt,
    incidentsQuery.dataUpdatedAt
  );

  return {
    components,
    groupedComponents,
    maintenances: sortedMaintenances,
    incidents,
    globalStatus,
    openIncidentCount,
    ongoingMaintenanceCount,
    isLoading,
    isFetching,
    error,
    refetch,
    dataUpdatedAt,
  };
}
