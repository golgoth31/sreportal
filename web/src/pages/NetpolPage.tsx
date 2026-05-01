import { useParams } from "react-router";
import { NetworkIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { ErrorAlert } from "@/components/ErrorAlert";
import { useNetpol } from "@/features/netpol/hooks/useNetpol";
import { FlowMatrixView } from "@/features/netpol/ui/FlowMatrixView";
import { FlowExplorerView } from "@/features/netpol/ui/FlowExplorerView";
import { CrossNamespaceView } from "@/features/netpol/ui/CrossNamespaceView";

export function NetpolPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const {
    graph,
    nodeMap,
    callsTo,
    callsFrom,
    allGroups,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useNetpol(portalName);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="font-display text-3xl tracking-tight">
          Network <span className="italic text-primary">policies</span>
        </h1>
        <div className="flex items-center gap-2 ml-auto">
          <PageRefreshButton onRefresh={refetch} isFetching={isFetching} />
          {graph && (
            <span className="text-muted-foreground text-sm font-mono">
              {graph.nodes.length} nodes · {graph.edges.length} edges
            </span>
          )}
        </div>
      </div>

      {error && <ErrorAlert title="Failed to load network policies" error={error} />}

      {isLoading && (
        <div className="py-16 text-center text-muted-foreground">Loading network policies...</div>
      )}

      {!isLoading && !error && !graph?.nodes.length && (
        <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
          <NetworkIcon className="size-8 text-muted-foreground" />
          <p className="text-muted-foreground text-sm">
            No network policies found for this portal.
          </p>
        </div>
      )}

      {graph && graph.nodes.length > 0 && !error && (
        <Tabs defaultValue="matrix">
          <TabsList variant="line">
            <TabsTrigger value="matrix">Flow Matrix</TabsTrigger>
            <TabsTrigger value="cross-pl">Cross-Namespace</TabsTrigger>
            <TabsTrigger value="impact">
              Flow Explorer
              <Badge variant="outline" className="ml-1.5 text-[10px] px-1.5 py-0">
                beta
              </Badge>
            </TabsTrigger>
          </TabsList>

          <TabsContent value="matrix">
            <FlowMatrixView
              nodes={graph.nodes}
              nodeMap={nodeMap}
              callsTo={callsTo}
              callsFrom={callsFrom}
              allGroups={allGroups}
            />
          </TabsContent>
          <TabsContent value="cross-pl">
            <CrossNamespaceView
              nodes={graph.nodes}
              edges={graph.edges}
              nodeMap={nodeMap}
            />
          </TabsContent>
          <TabsContent value="impact">
            <FlowExplorerView
              nodes={graph.nodes}
              nodeMap={nodeMap}
              callsTo={callsTo}
              callsFrom={callsFrom}
            />
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
