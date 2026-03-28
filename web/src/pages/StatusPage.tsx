import { useState } from "react";
import { useParams } from "react-router";

import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Skeleton } from "@/components/ui/skeleton";
import { useStatusPage } from "@/features/statuspage/hooks/useStatusPage";
import { StatusBanner } from "@/features/statuspage/ui/StatusBanner";
import { ComponentSection } from "@/features/statuspage/ui/ComponentSection";
import { MaintenanceSection } from "@/features/statuspage/ui/MaintenanceSection";
import { IncidentSection } from "@/features/statuspage/ui/IncidentSection";

type StatusTab = "components" | "incidents" | "maintenance";

export function StatusPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const [activeTab, setActiveTab] = useState<StatusTab>("components");
  const {
    groupedComponents,
    maintenances,
    incidents,
    globalStatus,
    openIncidentCount,
    ongoingMaintenanceCount,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useStatusPage({ portal: portalName });

  if (error) {
    return (
      <div className="max-w-screen-xl mx-auto px-4 py-6">
        <ErrorAlert title="Failed to load status page" error={error} />
      </div>
    );
  }

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="text-xl font-semibold tracking-tight">Status</h1>
        <PageRefreshButton onRefresh={refetch} isFetching={isFetching} />
      </div>
      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-16 w-full rounded-lg" />
          <Skeleton className="h-8 w-48" />
          <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-24 rounded-lg" />
            ))}
          </div>
        </div>
      ) : (
        <>
          <StatusBanner status={globalStatus} />

          {/* Tabs */}
          <div className="flex gap-1 border-b pb-px">
            <TabButton
              active={activeTab === "components"}
              onClick={() => setActiveTab("components")}
            >
              Components
            </TabButton>
            <TabButton
              active={activeTab === "incidents"}
              onClick={() => setActiveTab("incidents")}
            >
              Incidents
              {openIncidentCount > 0 && (
                <Badge variant="destructive" className="ml-1.5 text-[10px] px-1.5 py-0">
                  {openIncidentCount}
                </Badge>
              )}
            </TabButton>
            <TabButton
              active={activeTab === "maintenance"}
              onClick={() => setActiveTab("maintenance")}
            >
              Maintenance
              {ongoingMaintenanceCount > 0 && (
                <Badge variant="outline" className="ml-1.5 text-[10px] px-1.5 py-0 border-blue-500 text-blue-500">
                  {ongoingMaintenanceCount}
                </Badge>
              )}
            </TabButton>
          </div>

          {activeTab === "components" && (
            <ComponentSection groupedComponents={groupedComponents} />
          )}
          {activeTab === "incidents" && (
            <IncidentSection incidents={incidents} />
          )}
          {activeTab === "maintenance" && (
            <MaintenanceSection maintenances={maintenances} />
          )}
        </>
      )}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-sm font-medium rounded-t-md transition-colors ${
        active
          ? "bg-accent text-accent-foreground border-b-2 border-primary"
          : "text-muted-foreground hover:text-foreground hover:bg-muted"
      }`}
    >
      {children}
    </button>
  );
}
