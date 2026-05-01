import { ActivityIcon, AlertTriangleIcon, BarChart3Icon, ContainerIcon, LinkIcon, RocketIcon, ShieldIcon } from "lucide-react";
import { NavLink } from "react-router";

import { Badge } from "@/components/ui/badge";

import type { Portal } from "@/features/portal/domain/portal.types";
import { cn } from "@/lib/utils";

interface PortalSidebarProps {
  portalName: string;
  portals: Portal[];
}

export function PortalSidebar({
  portalName,
  portals,
}: PortalSidebarProps) {
  const currentPortal = portals.find(
    (p) => (p.subPath || p.name) === portalName
  );
  const basePath = currentPortal
    ? `/${currentPortal.subPath || currentPortal.name}`
    : `/${portalName}`;
  const showDNS = currentPortal?.features.dns !== false;
  const showReleases = currentPortal?.features.releases === true;
  const showNetworkPolicy = currentPortal?.features.networkPolicy !== false;
  const showAlerts = currentPortal?.features.alerts === true;
  const showStatusPage = currentPortal?.features.statusPage !== false;
  const showImageInventory = currentPortal?.features.imageInventory === true;

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      "relative flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
      "before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-4 before:w-[2px] before:rounded-r before:bg-primary before:opacity-0 before:transition-opacity",
      isActive
        ? "bg-primary/10 text-foreground before:opacity-100"
        : "text-muted-foreground hover:bg-accent hover:text-foreground"
    );

  return (
    <aside
      className="w-48 shrink-0 border-r border-border/60 bg-sidebar/40 flex flex-col py-4 overflow-y-auto"
      aria-label="Portal menu"
    >
      <div className="px-3 pb-3 mb-2 border-b border-border/60">
        <p className="text-[10px] font-mono uppercase tracking-[0.16em] text-muted-foreground">
          Resources
        </p>
      </div>
      <nav className="flex flex-col gap-0.5 px-2" aria-label="Links and Alerts">
        {showDNS && (
          <NavLink to={`${basePath}/links`} end className={linkClass}>
            <LinkIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>DNS</span>
          </NavLink>
        )}
        {showReleases && (
          <NavLink to={`${basePath}/releases`} className={linkClass}>
            <RocketIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Releases</span>
          </NavLink>
        )}
        {showNetworkPolicy && (
          <NavLink to={`${basePath}/netpol`} className={linkClass}>
            <ShieldIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Network Policies</span>
          </NavLink>
        )}
        {showAlerts && (
          <NavLink to={`${basePath}/alerts`} className={linkClass}>
            <AlertTriangleIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Alerts</span>
          </NavLink>
        )}
        {showStatusPage && (
          <NavLink to={`${basePath}/status`} className={linkClass}>
            <ActivityIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Status</span>
            <Badge variant="outline" className="ml-auto text-[10px] px-1.5 py-0 font-mono uppercase tracking-wider">alpha</Badge>
          </NavLink>
        )}
        {showImageInventory && (
          <NavLink to={`${basePath}/images`} className={linkClass}>
            <ContainerIcon className="size-4 shrink-0" aria-hidden="true" />
            <span>Images</span>
          </NavLink>
        )}
      </nav>
      <div className="mt-auto px-3 pt-3 mb-2 border-t border-border/60">
        <p className="text-[10px] font-mono uppercase tracking-[0.16em] text-muted-foreground">
          System
        </p>
      </div>
      <nav className="px-2" aria-label="Portal statistics">
        <NavLink to={`${basePath}/dashboard`} className={linkClass}>
          <BarChart3Icon className="size-4 shrink-0" aria-hidden="true" />
          <span>Statistics</span>
          <Badge variant="outline" className="ml-auto text-[10px] px-1.5 py-0 font-mono uppercase tracking-wider">beta</Badge>
        </NavLink>
      </nav>
    </aside>
  );
}
