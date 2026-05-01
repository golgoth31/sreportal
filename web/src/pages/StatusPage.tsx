import { useMatch, useParams } from "react-router";

import { ThemeToggle } from "@/components/ThemeToggle";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Skeleton } from "@/components/ui/skeleton";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useStatusPage } from "@/features/statuspage/hooks/useStatusPage";
import { StatusBanner } from "@/features/statuspage/ui/StatusBanner";
import { ComponentSection } from "@/features/statuspage/ui/ComponentSection";
import { MaintenanceSection } from "@/features/statuspage/ui/MaintenanceSection";
import { IncidentSection } from "@/features/statuspage/ui/IncidentSection";

export function StatusPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const isStandalone = useMatch("/status/:portalName") != null;
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
    dataUpdatedAt,
  } = useStatusPage({ portal: portalName });

  if (error) {
    return (
      <div className="max-w-screen-xl mx-auto px-4 py-6">
        <ErrorAlert title="Failed to load status page" error={error} />
      </div>
    );
  }

  const page = (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="font-display text-3xl tracking-tight">
          System <span className="italic text-primary">status</span>
        </h1>
        <div className="flex items-center gap-2 shrink-0">
          <PageRefreshButton onRefresh={refetch} isFetching={isFetching} />
          {isStandalone && <ThemeToggle />}
        </div>
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
          <StatusBanner status={globalStatus} dataUpdatedAt={dataUpdatedAt} />

          <Tabs defaultValue="components">
            <TabsList variant="line">
              <TabsTrigger value="components">Components</TabsTrigger>
              <TabsTrigger value="incidents" className="gap-1.5">
                Incidents
                {openIncidentCount > 0 && (
                  <Badge variant="destructive" className="text-[10px] px-1.5 py-0">
                    {openIncidentCount}
                  </Badge>
                )}
              </TabsTrigger>
              <TabsTrigger value="maintenance" className="gap-1.5">
                Maintenance
                {ongoingMaintenanceCount > 0 && (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0 border-primary/40 text-primary bg-primary/10">
                    {ongoingMaintenanceCount}
                  </Badge>
                )}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="components">
              <ComponentSection groupedComponents={groupedComponents} />
            </TabsContent>
            <TabsContent value="incidents">
              <IncidentSection incidents={incidents} />
            </TabsContent>
            <TabsContent value="maintenance">
              <MaintenanceSection maintenances={maintenances} />
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  );

  return isStandalone ? <TooltipProvider>{page}</TooltipProvider> : page;
}
