import { useQuery } from "@tanstack/react-query";

import { listReleases } from "../infrastructure/releaseApi";

/**
 * Returns whether releases exist at all.
 * Used to conditionally show the "Releases" sidebar link on the main portal.
 */
export function useHasReleases() {
  const query = useQuery({
    queryKey: ["has-releases"],
    queryFn: async () => {
      const data = await listReleases();
      return data.day !== "";
    },
    staleTime: 60_000,
  });

  return {
    hasReleases: query.data ?? false,
    isLoading: query.isLoading,
  };
}
