import { useQuery } from "@tanstack/react-query";

import { listDeployStatus } from "../infrastructure/deploystatusApi";
import type { DeployStatusEntry } from "../domain/deploystatus.types";

const EMPTY: DeployStatusEntry[] = [];

export function useDeployStatusQuery(portal: string) {
  const query = useQuery({
    queryKey: ["deploystatus", portal],
    queryFn: () => listDeployStatus(portal),
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
  return {
    entries: query.data ?? EMPTY,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
  };
}
