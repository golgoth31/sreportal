import { useQuery } from "@tanstack/react-query";

import { listAlerts } from "../infrastructure/alertmanagerApi";

/**
 * Returns the set of portal names (portalRef) that have at least one
 * Alertmanager resource. Used to show the "Alerts" link only for those portals.
 */
export function usePortalsWithAlerts() {
  const query = useQuery({
    queryKey: ["portals-with-alerts"],
    queryFn: async () => {
      const resources = await listAlerts({});
      const portalRefs = new Set(resources.map((r) => r.portalRef));
      return portalRefs;
    },
    staleTime: 30_000,
  });

  return {
    portalNamesWithAlerts: query.data ?? new Set<string>(),
    isLoading: query.isLoading,
    error: query.error,
  };
}
