import { AlertTriangleIcon, LinkIcon } from "lucide-react";
import { NavLink, useParams } from "react-router";

import { usePortals } from "@/features/portal/hooks/usePortals";
import { usePortalsWithAlerts } from "@/features/alertmanager/hooks/usePortalsWithAlerts";
import { cn } from "@/lib/utils";

export function PortalSidebar() {
  const { portalName } = useParams<{ portalName?: string }>();
  const { portals } = usePortals();
  const { portalNamesWithAlerts } = usePortalsWithAlerts();

  if (portalName == null) return null;

  const currentPortal = portals.find(
    (p) => (p.subPath || p.name) === portalName
  );
  const basePath = currentPortal
    ? `/${currentPortal.subPath || currentPortal.name}`
    : `/${portalName}`;
  const showAlerts =
    currentPortal != null && portalNamesWithAlerts.has(currentPortal.name);

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
          <LinkIcon className="size-4 shrink-0" aria-hidden />
          <span>Links</span>
        </NavLink>
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
            <AlertTriangleIcon className="size-4 shrink-0" aria-hidden />
            <span>Alerts</span>
          </NavLink>
        )}
      </nav>
    </aside>
  );
}
