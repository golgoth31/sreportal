import { useQuery } from "@tanstack/react-query";

import { listImages } from "../infrastructure/imageApi";
import type { Image } from "../domain/image.types";

const EMPTY: Image[] = [];

export function useImageQuery(portal: string) {
  const query = useQuery({
    queryKey: ["images", portal],
    queryFn: () => listImages(portal),
    staleTime: 30_000,
  });
  return {
    images: query.data ?? EMPTY,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
  };
}
