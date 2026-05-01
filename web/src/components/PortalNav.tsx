import { ExternalLinkIcon } from "lucide-react";
import { useCallback, useMemo } from "react";
import { NavLink, useNavigate, useParams } from "react-router";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import type { Portal } from "@/features/portal/domain/portal.types";
import { cn } from "@/lib/utils";

interface PortalNavProps {
  portals: Portal[];
  isLoading: boolean;
}

interface PortalSelectProps {
  portals: Portal[];
  activePortalName: string;
  placeholder: string;
  onSelect: (name: string) => void;
}

function PortalSelect({
  portals,
  activePortalName,
  placeholder,
  onSelect,
}: PortalSelectProps) {
  const sortedPortals = useMemo(
    () => [...portals].sort((a, b) => a.title.localeCompare(b.title)),
    [portals]
  );

  if (sortedPortals.length === 0) return null;

  const isActive = activePortalName !== "";

  return (
    <Select value={activePortalName} onValueChange={onSelect}>
      <SelectTrigger
        className={cn(
          "h-8 min-w-[10rem] gap-2 rounded-md border bg-background shadow-none transition-colors hover:bg-accent",
          "focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-offset-0",
          isActive
            ? "border-primary/40 bg-primary/8 text-primary font-semibold"
            : "border-border text-muted-foreground"
        )}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent position="popper">
        {sortedPortals.map((portal) => (
          <SelectItem key={portal.name} value={portal.name}>
            {portal.title}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

export function PortalNav({ portals, isLoading }: PortalNavProps) {
  const navigate = useNavigate();
  const { portalName } = useParams<{ portalName?: string }>();

  const mainPortal = portals.find((p) => p.main);
  const localPortals = portals.filter((p) => !p.main && !p.isRemote);
  const remotePortals = portals.filter((p) => p.isRemote);

  const activeLocalPortalName =
    localPortals.find((p) => (p.subPath || p.name) === portalName)?.name ?? "";
  const activeRemotePortal =
    remotePortals.find((p) => (p.subPath || p.name) === portalName);
  const activeRemotePortalName = activeRemotePortal?.name ?? "";

  const handlePortalSelect = useCallback(
    (name: string) => {
      const portal = portals.find((p) => p.name === name);
      if (portal) {
        void navigate(`/${portal.subPath || portal.name}/links`);
      }
    },
    [portals, navigate]
  );

  if (isLoading) {
    return (
      <div className="flex gap-2">
        <Skeleton className="h-8 w-20" />
        <Skeleton className="h-8 w-20" />
      </div>
    );
  }

  return (
    <nav
      className="flex items-center gap-3 flex-wrap"
      aria-label="Portal navigation"
    >
      {mainPortal && (
        <Button
          variant="ghost"
          size="sm"
          className={cn(
            "h-8 px-3",
            (mainPortal.subPath || mainPortal.name) === portalName &&
              "bg-primary/10 text-primary font-semibold"
          )}
          asChild
        >
          <NavLink to={`/${mainPortal.subPath || mainPortal.name}/links`}>
            {mainPortal.title}
          </NavLink>
        </Button>
      )}

      {localPortals.length > 0 && (
        <div className="flex items-center gap-1.5">
          <span className="text-[10px] font-mono uppercase tracking-[0.14em] text-muted-foreground">
            Local
          </span>
          <PortalSelect
            portals={localPortals}
            activePortalName={activeLocalPortalName}
            placeholder="Select…"
            onSelect={handlePortalSelect}
          />
        </div>
      )}

      {remotePortals.length > 0 && (
        <div className="flex items-center gap-1.5">
          <span className="text-[10px] font-mono uppercase tracking-[0.14em] text-muted-foreground">
            Remote
          </span>
          <PortalSelect
            portals={remotePortals}
            activePortalName={activeRemotePortalName}
            placeholder="Select…"
            onSelect={handlePortalSelect}
          />
          {activeRemotePortal?.url && (
            <Button variant="outline" size="sm" className="h-8" asChild>
              <a
                href={activeRemotePortal.url}
                target="_blank"
                rel="noopener noreferrer"
              >
                Open
                <ExternalLinkIcon className="size-3" />
              </a>
            </Button>
          )}
        </div>
      )}
    </nav>
  );
}
