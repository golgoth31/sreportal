import { HelpCircleIcon, NetworkIcon } from "lucide-react";
import { NavLink, Outlet } from "react-router";

import { Button } from "@/components/ui/button";
import { Toaster } from "@/components/ui/sonner";
import { usePortals } from "@/features/portal/hooks/usePortals";
import { cn } from "@/lib/utils";
import { PortalNav } from "./PortalNav";
import { ThemeToggle } from "./ThemeToggle";

export function RootLayout() {
  const { portals, isLoading } = usePortals();

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* Header */}
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur-sm">
        <div className="flex h-14 items-center gap-4 px-4 max-w-screen-xl mx-auto w-full">
          {/* Brand */}
          <div className="flex items-center gap-2 font-semibold text-sm shrink-0">
            <NetworkIcon className="size-5 text-primary" />
            <span>SRE Portal</span>
          </div>

          {/* Portal navigation */}
          <div className="flex-1 min-w-0 overflow-x-auto">
            <PortalNav portals={portals} isLoading={isLoading} />
          </div>

          {/* Right actions */}
          <div className="flex items-center gap-1 shrink-0">
            <NavLink to="/help">
              {({ isActive }) => (
                <Button
                  variant="ghost"
                  size="icon"
                  className={cn(isActive && "bg-accent")}
                  aria-label="Help and MCP integration"
                  asChild
                >
                  <span>
                    <HelpCircleIcon className="size-4" />
                  </span>
                </Button>
              )}
            </NavLink>
            <ThemeToggle />
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="flex-1">
        <Outlet />
      </main>

      {/* Footer */}
      <footer className="border-t py-4 px-4">
        <p className="text-center text-xs text-muted-foreground">
          SRE Portal — Kubernetes DNS Management
        </p>
      </footer>

      <Toaster position="bottom-right" richColors />
    </div>
  );
}
