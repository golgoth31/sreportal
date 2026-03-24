import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { listNetworkPolicies } from "../infrastructure/netpolApi";
import { buildFlowMaps, type NetpolNode } from "../domain/netpol.types";

export function useNetpol() {
  const [search, setSearch] = useState("");

  const query = useQuery({
    queryKey: ["netpol"],
    queryFn: () => listNetworkPolicies(),
    staleTime: 30_000,
  });

  const graph = query.data;

  const nodeMap = useMemo(() => {
    if (!graph) return new Map<string, NetpolNode>();
    return new Map(graph.nodes.map((n) => [n.id, n]));
  }, [graph]);

  const { callsTo, callsFrom } = useMemo(() => {
    if (!graph) return { callsTo: new Map(), callsFrom: new Map() };
    return buildFlowMaps(graph.edges);
  }, [graph]);

  const allGroups = useMemo(() => {
    if (!graph) return [];
    return [...new Set(graph.nodes.map((n) => n.group))].sort();
  }, [graph]);

  const allNodeTypes = useMemo(() => {
    if (!graph) return [];
    return [...new Set(graph.nodes.map((n) => n.nodeType))].sort();
  }, [graph]);

  return {
    graph,
    nodeMap,
    callsTo,
    callsFrom,
    allGroups,
    allNodeTypes,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error,
    refetch: query.refetch,
    search,
    setSearch,
  };
}
