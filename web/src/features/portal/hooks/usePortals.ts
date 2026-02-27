import { useQuery } from "@tanstack/react-query";

import { listPortals } from "../infrastructure/portalApi";

export function usePortals() {
  const query = useQuery({
    queryKey: ["portals"],
    queryFn: listPortals,
    staleTime: 30_000,
  });

  return {
    portals: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error,
  };
}
