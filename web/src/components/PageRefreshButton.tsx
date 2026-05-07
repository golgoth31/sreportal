import { RefreshCwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface PageRefreshButtonProps {
  onRefresh: () => void | Promise<unknown>;
  isFetching?: boolean;
  disabled?: boolean;
  className?: string;
  /** Visible label; keep short for toolbars */
  label?: string;
}

export function PageRefreshButton({
  onRefresh,
  isFetching = false,
  disabled = false,
  className,
  label = "Refresh",
}: PageRefreshButtonProps) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className={cn("gap-2 shrink-0", className)}
      onClick={() => void onRefresh()}
      disabled={disabled || isFetching}
    >
      <RefreshCwIcon
        className={cn("size-4", isFetching && "animate-spin")}
        aria-hidden
      />
      {label}
    </Button>
  );
}
