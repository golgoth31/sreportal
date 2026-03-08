import { useQuery } from "@tanstack/react-query";

import { getVersion } from "../infrastructure/versionApi";

export function useVersion() {
  const query = useQuery({
    queryKey: ["version"],
    queryFn: getVersion,
    staleTime: Infinity,
  });

  return {
    version: query.data?.version ?? null,
    commit: query.data?.commit ?? null,
    date: query.data?.date ?? null,
    isLoading: query.isLoading,
  };
}
