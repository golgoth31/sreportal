import { AlertTriangleIcon } from "lucide-react";

interface RemoteSyncStaleBannerProps {
  /** Controller-reported sync error (e.g. connection failure). */
  lastSyncError: string;
}

/**
 * Warns that remote portal data may be stale when lastSyncError is set on the Portal CR.
 */
export function RemoteSyncStaleBanner({ lastSyncError }: RemoteSyncStaleBannerProps) {
  const detail = lastSyncError.trim();
  if (!detail) return null;

  return (
    <div
      role="alert"
      className="border-b border-amber-500/40 bg-amber-500/10 px-4 py-3 text-amber-950 dark:text-amber-50"
    >
      <div className="max-w-screen-xl mx-auto flex gap-3">
        <AlertTriangleIcon
          className="size-5 shrink-0 text-amber-600 dark:text-amber-400"
          aria-hidden
        />
        <div className="min-w-0 space-y-1">
          <p className="text-sm font-medium">
            Synchronization failed — data below may be out of date
          </p>
          <p className="text-xs text-amber-900/90 dark:text-amber-100/90">
            What you see may not reflect the current state of the selected portal.
          </p>
        </div>
      </div>
    </div>
  );
}
