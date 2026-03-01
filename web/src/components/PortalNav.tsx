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
  if (portals.length === 0) return null;

  return (
    <Select value={activePortalName} onValueChange={onSelect}>
      <SelectTrigger
        className={cn(
          "h-8 min-w-[8rem] border-none bg-transparent shadow-none hover:bg-accent focus-visible:border-transparent focus-visible:ring-0 dark:bg-transparent",
          activePortalName && "bg-primary/10 text-primary font-semibold"
        )}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {portals.map((portal) => (
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
  const activeRemotePortalName =
    remotePortals.find((p) => (p.subPath || p.name) === portalName)?.name ?? "";

  function handlePortalSelect(name: string) {
    const portal = portals.find((p) => p.name === name);
    if (portal) {
      void navigate(`/${portal.subPath || portal.name}/links`);
    }
  }

  if (isLoading) {
    return (
      <div className="flex gap-2">
        <Skeleton className="h-8 w-20" />
        <Skeleton className="h-8 w-20" />
      </div>
    );
  }

  return (
    <nav className="flex items-center gap-1" aria-label="Portal navigation">
      {mainPortal && (
        <NavLink to={`/${mainPortal.subPath || mainPortal.name}/links`}>
          {({ isActive }) => (
            <Button
              variant="ghost"
              size="sm"
              className={cn(
                isActive && "bg-primary/10 text-primary font-semibold shadow-primary"
              )}
              asChild
            >
              <span>{mainPortal.title}</span>
            </Button>
          )}
        </NavLink>
      )}

      <PortalSelect
        portals={localPortals}
        activePortalName={activeLocalPortalName}
        placeholder="Local portals"
        onSelect={handlePortalSelect}
      />

      <PortalSelect
        portals={remotePortals}
        activePortalName={activeRemotePortalName}
        placeholder="Remote portals"
        onSelect={handlePortalSelect}
      />
    </nav>
  );
}
