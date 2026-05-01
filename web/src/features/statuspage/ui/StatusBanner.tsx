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
    <div role="status" className={cn("rounded-lg border p-5", colorClass)}>
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <StatusDot status={status} />
          <span className="font-display italic text-2xl tracking-tight">{message}</span>
        </div>
        {dataUpdatedAt > 0 && (
          <span className="text-[11px] font-mono uppercase tracking-wider opacity-70">
            Updated · {new Date(dataUpdatedAt).toLocaleString()}
          </span>
        )}
      </div>
    </div>
  );
}

const STATUS_DOT_COLORS: Record<ComponentStatus, string> = {
  operational: "bg-emerald-500 shadow-[0_0_10px_oklch(0.7_0.18_152/0.6)]",
  degraded: "bg-amber-500 shadow-[0_0_10px_oklch(0.75_0.16_70/0.6)]",
  partial_outage: "bg-orange-500 shadow-[0_0_10px_oklch(0.7_0.18_50/0.6)]",
  major_outage: "bg-rose-500 shadow-[0_0_10px_oklch(0.65_0.22_22/0.7)]",
  maintenance: "bg-primary shadow-[0_0_10px_oklch(0.7_0.18_277/0.6)]",
  unknown: "bg-muted-foreground",
};

function StatusDot({ status }: { status: ComponentStatus }) {
  return (
    <span
      className={cn("inline-block size-3.5 rounded-full", STATUS_DOT_COLORS[status])}
      aria-hidden="true"
    />
  );
}
