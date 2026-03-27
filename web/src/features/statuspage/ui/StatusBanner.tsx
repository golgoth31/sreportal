import type { ComponentStatus } from "../domain/types";
import {
  getStatusColor,
  getGlobalStatusMessage,
} from "../domain/utils";

interface StatusBannerProps {
  status: ComponentStatus;
}

export function StatusBanner({ status }: StatusBannerProps) {
  const colorClass = getStatusColor(status);
  const message = getGlobalStatusMessage(status);

  return (
    <div className={`rounded-lg border p-4 ${colorClass}`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <StatusDot status={status} />
          <span className="text-lg font-semibold">{message}</span>
        </div>
        <span className="text-xs opacity-70">
          Last updated: {new Date().toLocaleString()}
        </span>
      </div>
    </div>
  );
}

function StatusDot({ status }: { status: ComponentStatus }) {
  const dotColor =
    status === "operational"
      ? "bg-green-500"
      : status === "degraded"
        ? "bg-yellow-500"
        : status === "partial_outage"
          ? "bg-orange-500"
          : status === "major_outage"
            ? "bg-red-500"
            : status === "maintenance"
              ? "bg-blue-500"
              : "bg-gray-400";

  return (
    <span
      className={`inline-block size-3 rounded-full ${dotColor}`}
      aria-hidden="true"
    />
  );
}
