import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { format } from "date-fns";

import { DEFAULT_TYPE_COLORS } from "../domain/release.types";
import { listReleaseDays } from "../infrastructure/releaseApi";

const DAY_FORMAT = "yyyy-MM-dd";

export function useReleaseDays(portal: string) {
  const query = useQuery({
    queryKey: ["releaseDays", portal],
    queryFn: () => listReleaseDays(portal),
    staleTime: 30_000,
  });

  const data = query.data;

  const daysSet = useMemo(
    () => new Set(data?.days ?? []),
    [data?.days],
  );

  const ttlDays = data?.ttlDays ?? 0;
  const serverTypes = data?.types ?? [];
  const types = serverTypes.length > 0 ? serverTypes : DEFAULT_TYPE_COLORS;

  const isDayDisabled = useCallback(
    (date: Date): boolean => {
      const key = format(date, DAY_FORMAT);
      return !daysSet.has(key);
    },
    [daysSet],
  );

  return {
    daysSet,
    ttlDays,
    types,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    isDayDisabled,
    refetch: query.refetch,
  };
}
