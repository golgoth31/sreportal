import { cn } from "@/lib/utils";

import type { ComponentStatus } from "../domain/types";
import { getStatusColor, getGlobalStatusMessage } from "../domain/utils";

interface StatusBannerProps {
  status: ComponentStatus;
  dataUpdatedAt: number;
}

export function StatusBanner({ status, dataUpdatedAt }: StatusBannerProps) {
  const colorClass = getStatusColor(status);
  const message = getGlobalStatusMessage(status);

  return (
    <div role="status" className={cn("rounded-lg border p-4", colorClass)}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <StatusDot status={status} />
          <span className="text-lg font-semibold">{message}</span>
        </div>
        {dataUpdatedAt > 0 && (
          <span className="text-xs opacity-70">
            Last updated: {new Date(dataUpdatedAt).toLocaleString()}
          </span>
        )}
      </div>
    </div>
  );
}

const STATUS_DOT_COLORS: Record<ComponentStatus, string> = {
  operational: "bg-green-500",
  degraded: "bg-yellow-500",
  partial_outage: "bg-orange-500",
  major_outage: "bg-red-500",
  maintenance: "bg-blue-500",
  unknown: "bg-gray-400",
};

function StatusDot({ status }: { status: ComponentStatus }) {
  return (
    <span
      className={cn("inline-block size-3 rounded-full", STATUS_DOT_COLORS[status])}
      aria-hidden="true"
    />
  );
}
