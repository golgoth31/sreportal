import { useMemo } from "react";

import { PageRefreshButton } from "@/components/PageRefreshButton";
import {
  ActivityIcon,
  GlobeIcon,
  NetworkIcon,
  RadioIcon,
  ShieldAlertIcon,
  WaypointsIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import {
  extractDashboardStats,
  extractFqdnBarData,
  extractPortalDonutData,
} from "@/features/metrics/domain/dashboard.types";
import { useMetrics } from "@/features/metrics/hooks/useMetrics";
import { FqdnBarChart } from "@/features/metrics/ui/FqdnBarChart";
import { MetricList } from "@/features/metrics/ui/MetricList";
import { PortalDonutChart } from "@/features/metrics/ui/PortalDonutChart";
import { StatCard } from "@/features/metrics/ui/StatCard";

export function DashboardPage() {
  const {
    families,
    allFamilies,
    isLoading,
    isFetching,
    error,
    hasFilters,
    clearFilters,
    refetch,
  } = useMetrics();

  const stats = useMemo(
    () => extractDashboardStats(allFamilies),
    [allFamilies],
  );

  const barData = useMemo(
    () => extractFqdnBarData(allFamilies),
    [allFamilies],
  );

  const donutData = useMemo(
    () => extractPortalDonutData(allFamilies),
    [allFamilies],
  );

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold tracking-tight">Portal Statistics</h1>
          <Badge variant="outline">beta</Badge>
        </div>
        <PageRefreshButton
          className="ml-auto"
          onRefresh={refetch}
          isFetching={isFetching}
        />
      </div>

      {error && <ErrorAlert title="Failed to load metrics" error={error} />}

      {/* Row 1 — Stat Cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <StatCard
          title="Total FQDNs"
          value={stats.totalFqdns}
          icon={GlobeIcon}
        />
        <StatCard
          title="Active Alerts"
          value={stats.activeAlerts}
          icon={ShieldAlertIcon}
        />
        <StatCard
          title="Portals"
          value={stats.totalPortals}
          icon={NetworkIcon}
        />
        <StatCard
          title="HTTP In-Flight"
          value={stats.httpInFlight}
          icon={ActivityIcon}
        />
        <StatCard
          title="MCP Sessions"
          value={stats.mcpSessions}
          icon={RadioIcon}
        />
        <StatCard
          title="Source Endpoints"
          value={stats.sourceEndpoints}
          icon={WaypointsIcon}
        />
      </div>

      {/* Row 2 — Charts */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="min-w-0 md:col-span-2">
          <FqdnBarChart data={barData} />
        </div>
        <div className="min-w-0">
          <PortalDonutChart data={donutData} />
        </div>
      </div>

      {/* Row 3 — Metric Families */}
      {!error && (
        <MetricList
          families={families}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearFilters}
        />
      )}
    </div>
  );
}
