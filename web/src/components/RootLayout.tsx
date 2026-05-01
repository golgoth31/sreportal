import { HelpCircleIcon } from "lucide-react";
import { NavLink, Outlet, useParams } from "react-router";

import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { hasRemoteSyncError } from "@/features/portal/domain/portal.types";
import { usePortals } from "@/features/portal/hooks/usePortals";
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
        <header className="sticky top-0 z-40 border-b border-border/60 bg-background/80 backdrop-blur-md supports-[backdrop-filter]:bg-background/60">
          <div className="flex h-14 items-center gap-4 px-4 max-w-screen-xl mx-auto w-full">
            {/* Brand */}
            <div className="flex items-center gap-2 shrink-0">
              <SrePortalIcon className="size-5" />
              <span className="font-display text-lg leading-none tracking-tight">
                SRE <span className="italic text-primary">Portal</span>
              </span>
            </div>
            <span className="h-6 w-px bg-border/80 shrink-0" aria-hidden="true" />

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
        <footer className="border-t border-border/60 py-3 px-4">
          <p className="text-center text-[11px] font-mono uppercase tracking-[0.14em] text-muted-foreground">
            SRE Portal{version ? ` · ${version}` : ""}
          </p>
        </footer>

        <Toaster position="top-center" richColors />
      </div>
    </TooltipProvider>
  );
}
