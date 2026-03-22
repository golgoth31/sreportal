import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { format } from "date-fns";

import { listReleaseDays } from "../infrastructure/releaseApi";

const DAY_FORMAT = "yyyy-MM-dd";

export function useReleaseDays() {
  const query = useQuery({
    queryKey: ["releaseDays"],
    queryFn: listReleaseDays,
    staleTime: 30_000,
  });

  const data = query.data;

  const daysSet = useMemo(
    () => new Set(data?.days ?? []),
    [data?.days],
  );

  const ttlDays = data?.ttlDays ?? 0;

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
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    isDayDisabled,
    refetch: query.refetch,
  };
}
