import { useState } from "react";
import { useParams } from "react-router";

import { Badge } from "@/components/ui/badge";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { ErrorAlert } from "@/components/ErrorAlert";
import { useNetpol } from "@/features/netpol/hooks/useNetpol";
import { FlowMatrixView } from "@/features/netpol/ui/FlowMatrixView";
import { ImpactView } from "@/features/netpol/ui/ImpactView";
import { CrossNamespaceView } from "@/features/netpol/ui/CrossNamespaceView";

type ViewTab = "matrix" | "cross-pl" | "impact";

const TABS: { value: ViewTab; label: string }[] = [
  { value: "matrix", label: "Flow Matrix" },
  { value: "cross-pl", label: "Cross-Namespace" },
  { value: "impact", label: "Impact" },
];

export function NetpolPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const [activeTab, setActiveTab] = useState<ViewTab>("matrix");
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
        <h1 className="text-xl font-semibold tracking-tight">Network Policies</h1>
        <div className="flex items-center gap-2 ml-auto">
          <PageRefreshButton onRefresh={refetch} isFetching={isFetching} />
          {graph && (
            <span className="text-muted-foreground text-sm">
              {graph.nodes.length} nodes, {graph.edges.length} edges
            </span>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b pb-px">
        {TABS.map((tab) => (
          <button
            key={tab.value}
            onClick={() => setActiveTab(tab.value)}
            className={`px-4 py-2 text-sm font-medium rounded-t-md transition-colors ${
              activeTab === tab.value
                ? "bg-accent text-accent-foreground border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            {tab.label}
            {tab.value === "impact" && (
              <Badge variant="outline" className="ml-1.5 text-[10px] px-1.5 py-0">beta</Badge>
            )}
          </button>
        ))}
      </div>

      {error && <ErrorAlert title="Failed to load network policies" error={error} />}

      {isLoading && (
        <div className="py-16 text-center text-muted-foreground">Loading network policies...</div>
      )}

      {graph && !error && (
        <>
          {activeTab === "matrix" && (
            <FlowMatrixView
              nodes={graph.nodes}
              nodeMap={nodeMap}
              callsTo={callsTo}
              callsFrom={callsFrom}
              allGroups={allGroups}
            />
          )}
          {activeTab === "cross-pl" && (
            <CrossNamespaceView
              nodes={graph.nodes}
              edges={graph.edges}
              nodeMap={nodeMap}
            />
          )}
          {activeTab === "impact" && (
            <ImpactView
              nodes={graph.nodes}
              nodeMap={nodeMap}
              callsFrom={callsFrom}
            />
          )}
        </>
      )}
    </div>
  );
}
