import { AlertTriangleIcon, BarChart3Icon, LinkIcon, RocketIcon } from "lucide-react";
import { NavLink } from "react-router";

import { Badge } from "@/components/ui/badge";

import type { Portal } from "@/features/portal/domain/portal.types";
import { cn } from "@/lib/utils";

interface PortalSidebarProps {
  portalName: string;
  portals: Portal[];
  portalNamesWithAlerts: ReadonlySet<string>;
  hasReleases: boolean;
}

export function PortalSidebar({
  portalName,
  portals,
  portalNamesWithAlerts,
  hasReleases,
}: PortalSidebarProps) {
  const currentPortal = portals.find(
    (p) => (p.subPath || p.name) === portalName
  );
  const basePath = currentPortal
    ? `/${currentPortal.subPath || currentPortal.name}`
    : `/${portalName}`;
  const showAlerts =
    currentPortal != null && portalNamesWithAlerts.has(currentPortal.name);
  const showReleases = currentPortal?.main === true && hasReleases;

  return (
    <aside
      className="w-48 shrink-0 border-r bg-muted/30 flex flex-col py-4"
      aria-label="Portal menu"
    >
      <nav className="flex flex-col gap-0.5 px-2" aria-label="Links and Alerts">
        <NavLink
          to={`${basePath}/links`}
          end
          className={({ isActive }) =>
            cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )
          }
        >
          <LinkIcon className="size-4 shrink-0" aria-hidden="true" />
          <span>DNS</span>
        </NavLink>
        {showReleases && (
          <NavLink
            to={`${basePath}/releases`}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )
            }
          >
            <RocketIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Releases</span>
          </NavLink>
        )}
        {showAlerts && (
          <NavLink
            to={`${basePath}/alerts`}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )
            }
          >
            <AlertTriangleIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Alerts</span>
          </NavLink>
        )}
      </nav>
      <nav className="mt-auto px-2" aria-label="Portal statistics">
        <NavLink
          to={`${basePath}/dashboard`}
          className={({ isActive }) =>
            cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )
          }
        >
          <BarChart3Icon className="size-4 shrink-0" aria-hidden="true" />
          <span>Portal Statistics</span>
          <Badge variant="outline" className="ml-auto text-[10px] px-1.5 py-0">beta</Badge>
        </NavLink>
      </nav>
    </aside>
  );
}
