import { ExternalLinkIcon } from "lucide-react";
import { NavLink } from "react-router";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { Portal } from "@/features/portal/domain/portal.types";
import { cn } from "@/lib/utils";

interface PortalNavProps {
  portals: Portal[];
  isLoading: boolean;
}

export function PortalNav({ portals, isLoading }: PortalNavProps) {
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
      {portals.map((portal) => {
        const path = `/${portal.subPath || portal.name}/links`;

        if (portal.isRemote && portal.url) {
          return (
            <Button
              key={portal.name}
              variant="ghost"
              size="sm"
              asChild
            >
              <a
                href={portal.url}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={`${portal.title} (opens remote portal)`}
              >
                {portal.title}
                <ExternalLinkIcon className="size-3 text-muted-foreground" />
              </a>
            </Button>
          );
        }

        return (
          <NavLink key={portal.name} to={path}>
            {({ isActive }) => (
              <Button
                variant="ghost"
                size="sm"
                className={cn(
                  isActive &&
                  "bg-primary/10 text-primary font-semibold shadow-primary"
                )}
                asChild
              >
                <span>{portal.title}</span>
              </Button>
            )}
          </NavLink>
        );
      })}
    </nav>
  );
}
