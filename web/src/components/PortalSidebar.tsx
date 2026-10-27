import {
  ActivityIcon,
  AlertTriangleIcon,
  BarChart3Icon,
  ChevronsLeftIcon,
  ChevronsRightIcon,
  ContainerIcon,
  LinkIcon,
  RocketIcon,
  ShieldIcon,
  type LucideIcon,
} from "lucide-react";
import { NavLink } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useSidebarCollapsed } from "@/hooks/useSidebarCollapsed";

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
  const { collapsed, toggle } = useSidebarCollapsed();
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

  const itemLayout = collapsed ? "justify-center px-2" : "gap-3 px-3";
  const sectionPadding = collapsed ? "px-2" : "px-3";
  const ToggleIcon = collapsed ? ChevronsRightIcon : ChevronsLeftIcon;

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      "relative flex items-center rounded-md py-2 text-sm font-medium transition-colors",
      itemLayout,
      "before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-4 before:w-[3px] before:rounded-r before:bg-primary before:opacity-0 before:transition-opacity",
      isActive
        ? "bg-primary/10 text-foreground before:opacity-100"
        : "text-muted-foreground hover:bg-accent hover:text-foreground"
    );

  const renderItem = (
    to: string,
    label: string,
    Icon: LucideIcon,
    opts: { end?: boolean; badge?: string } = {},
  ) => {
    let badge = null;
    if (opts.badge && collapsed) {
      badge = (
        <span
          className="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-primary"
          aria-hidden="true"
          data-testid="sidebar-badge-dot"
        />
      );
    } else if (opts.badge) {
      badge = (
        <Badge
          variant="outline"
          className="ml-auto text-[10px] px-1.5 py-0 font-mono uppercase tracking-wider"
        >
          {opts.badge}
        </Badge>
      );
    }
    const link = (
      <NavLink to={to} end={opts.end} className={linkClass}>
        <Icon className="size-4 shrink-0" aria-hidden="true" />
        <span className={collapsed ? "sr-only" : undefined}>{label}</span>
        {badge}
      </NavLink>
    );
    if (!collapsed) return link;
    return (
      <Tooltip>
        <TooltipTrigger asChild>{link}</TooltipTrigger>
        <TooltipContent side="right">
          {label}
          {opts.badge ? ` (${opts.badge})` : ""}
        </TooltipContent>
      </Tooltip>
    );
  };

  const sectionTitleClass = cn(
    "text-[10px] font-mono uppercase tracking-[0.08em] text-muted-foreground",
    collapsed && "sr-only",
  );

  return (
    <TooltipProvider>
      <aside
        className={cn(
          "shrink-0 border-r border-border bg-sidebar flex flex-col py-4 overflow-y-auto transition-[width]",
          collapsed ? "w-[var(--sidebar-width-collapsed)]" : "w-[var(--sidebar-width)]",
        )}
        aria-label="Portal menu"
      >
        <div className={cn("pb-3 mb-2 border-b border-border", sectionPadding)}>
          <p className={sectionTitleClass}>Resources</p>
        </div>
        <nav className="flex flex-col gap-0.5 px-2" aria-label="Links and Alerts">
          {showDNS && renderItem(`${basePath}/links`, "DNS", LinkIcon, { end: true })}
          {showReleases && renderItem(`${basePath}/releases`, "Releases", RocketIcon)}
          {showNetworkPolicy && renderItem(`${basePath}/netpol`, "Network Policies", ShieldIcon)}
          {showAlerts && renderItem(`${basePath}/alerts`, "Alerts", AlertTriangleIcon)}
          {showStatusPage &&
            renderItem(`${basePath}/status`, "Status", ActivityIcon, { badge: "alpha" })}
          {showImageInventory && renderItem(`${basePath}/images`, "Images", ContainerIcon)}
        </nav>
        <div className={cn("mt-auto pt-3 mb-2 border-t border-border", sectionPadding)}>
          <p className={sectionTitleClass}>System</p>
        </div>
        <nav className="px-2" aria-label="Portal statistics">
          {renderItem(`${basePath}/dashboard`, "Statistics", BarChart3Icon, { badge: "beta" })}
        </nav>
        <div className="px-2 pt-2">
          <button
            type="button"
            className={cn(
              "flex w-full items-center rounded-md py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
              itemLayout,
            )}
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            onClick={toggle}
          >
            <ToggleIcon className="size-4 shrink-0" aria-hidden="true" />
            {!collapsed && <span>Collapse</span>}
          </button>
        </div>
      </aside>
    </TooltipProvider>
  );
}
