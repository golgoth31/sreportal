import { useQuery } from "@tanstack/react-query";

import { listFqdns } from "../infrastructure/dnsApi";
import type { Fqdn } from "../domain/dns.types";

const EMPTY_FQDNS: Fqdn[] = [];

export function useDnsQuery(portal: string) {
  const query = useQuery({
    queryKey: ["fqdns", portal],
    queryFn: () => listFqdns(portal),
  });

  return {
    fqdns: query.data ?? EMPTY_FQDNS,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
  };
}
