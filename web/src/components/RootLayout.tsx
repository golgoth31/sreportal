import { HelpCircleIcon } from "lucide-react";
import { NavLink, Outlet, useParams } from "react-router";

import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { usePortalsWithAlerts } from "@/features/alertmanager/hooks/usePortalsWithAlerts";
import { hasRemoteSyncError } from "@/features/portal/domain/portal.types";
import { usePortals } from "@/features/portal/hooks/usePortals";
import { useHasReleases } from "@/features/release/hooks/useHasReleases";
import { RemoteSyncStaleBanner } from "@/features/portal/ui/RemoteSyncStaleBanner";
import { useVersion } from "@/features/version/hooks/useVersion";
import { cn } from "@/lib/utils";
import { PortalNav } from "./PortalNav";
import { PortalSidebar } from "./PortalSidebar";
import { SrePortalIcon } from "./SrePortalIcon";
import { ThemeToggle } from "./ThemeToggle";

export function RootLayout() {
  const { portals, isLoading } = usePortals();
  const { version } = useVersion();
  const { portalNamesWithAlerts } = usePortalsWithAlerts();
  const { hasReleases } = useHasReleases();

  const { portalName } = useParams<{ portalName?: string }>();
  const showSidebar = portalName != null;
  const currentPortal =
    portalName != null
      ? portals.find((p) => (p.subPath || p.name) === portalName)
      : undefined;
  const showRemoteSyncWarning =
    currentPortal != null && hasRemoteSyncError(currentPortal);

  return (
    <TooltipProvider>
      <div className="h-screen flex flex-col bg-background">
        {/* Header */}
        <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur-sm">
          <div className="flex h-14 items-center gap-4 px-4 max-w-screen-xl mx-auto w-full">
            {/* Brand */}
            <div className="flex items-center gap-2 font-semibold text-sm shrink-0">
              <SrePortalIcon className="size-5" />
              <span>SRE Portal</span>
            </div>

            {/* Portal navigation */}
            <div className="flex-1 min-w-0 overflow-x-auto">
              <PortalNav portals={portals} isLoading={isLoading} />
            </div>

            {/* Right actions */}
            <div className="flex items-center gap-1 shrink-0">
              <NavLink
                to="/help"
                className={({ isActive }) =>
                  cn(
                    "inline-flex items-center justify-center rounded-md size-9 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
                    isActive && "bg-accent"
                  )
                }
                aria-label="Help and MCP integration"
              >
                <HelpCircleIcon className="size-4" />
              </NavLink>
              <ThemeToggle />
            </div>
          </div>
        </header>

        {/* Content area: sidebar + main */}
        <div className="flex flex-1 min-h-0">
          {showSidebar && (
            <PortalSidebar
              portalName={portalName}
              portals={portals}
              portalNamesWithAlerts={portalNamesWithAlerts}
              hasReleases={hasReleases}
            />
          )}
          <main className="flex-1 min-w-0 overflow-auto flex flex-col">
            {showRemoteSyncWarning && currentPortal?.remoteSync && (
              <RemoteSyncStaleBanner
                lastSyncError={currentPortal.remoteSync.lastSyncError}
              />
            )}
            <div className="flex-1 min-h-0">
              <Outlet />
            </div>
          </main>
        </div>

        {/* Footer */}
        <footer className="border-t py-4 px-4">
          <p className="text-center text-xs text-muted-foreground">
            SRE Portal{version ? ` — ${version}` : ""}
          </p>
        </footer>

        <Toaster position="top-center" richColors />
      </div>
    </TooltipProvider>
  );
}
