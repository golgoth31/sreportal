import { useParams } from "react-router";

import { ErrorAlert } from "@/components/ErrorAlert";
import { PageRefreshButton } from "@/components/PageRefreshButton";
import { useDeployStatusQuery } from "@/features/deploystatus/hooks/useDeployStatusQuery";
import { DeployStatusList } from "@/features/deploystatus/ui/DeployStatusList";

export function DeployStatusPage() {
  const { portalName = "main" } = useParams<{ portalName: string }>();
  const { entries, isLoading, isFetching, error, refetch } =
    useDeployStatusQuery(portalName);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="font-display text-3xl tracking-tight">
          Deploy <span className="italic text-primary">status</span>
        </h1>
        <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">
          <PageRefreshButton onRefresh={() => void refetch()} isFetching={isFetching} />
          {!isLoading && !error && (
            <span className="text-muted-foreground text-sm font-mono">
              {entries.length} service{entries.length !== 1 ? "s" : ""}
            </span>
          )}
        </div>
      </div>

      {error && <ErrorAlert title="Failed to load deploy status" error={error} />}

      {!error && <DeployStatusList entries={entries} isLoading={isLoading} />}
    </div>
  );
}
