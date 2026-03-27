import { useParams } from "react-router";

import { ErrorAlert } from "@/components/ErrorAlert";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Skeleton } from "@/components/ui/skeleton";
import { useStatusPage } from "@/features/statuspage/hooks/useStatusPage";
import { StatusBanner } from "@/features/statuspage/ui/StatusBanner";
import { ComponentSection } from "@/features/statuspage/ui/ComponentSection";
import { MaintenanceSection } from "@/features/statuspage/ui/MaintenanceSection";
import { IncidentSection } from "@/features/statuspage/ui/IncidentSection";

export function StatusPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const {
    groupedComponents,
    maintenances,
    incidents,
    globalStatus,
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
          <ComponentSection groupedComponents={groupedComponents} />
          <MaintenanceSection maintenances={maintenances} />
          <IncidentSection incidents={incidents} />
        </>
      )}
    </div>
  );
}
