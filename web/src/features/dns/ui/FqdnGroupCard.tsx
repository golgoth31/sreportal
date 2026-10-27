import { ChevronDownIcon, DatabaseIcon, PencilIcon } from "lucide-react";

import { TableCell, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type { FqdnGroup } from "../domain/dns.types";
import { FqdnCard } from "./FqdnCard";

export const FQDN_TABLE_COL_COUNT = 6;

interface FqdnGroupCardProps {
  group: FqdnGroup;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Group header + data rows for a parent page-level Soft Material table. */
export function FqdnGroupCard({ group, open, onOpenChange }: FqdnGroupCardProps) {
  const isManual = group.source === "manual";
  const SourceIcon = isManual ? PencilIcon : DatabaseIcon;

  return (
    <>
      <TableRow className="hover:bg-muted/40 bg-gradient-to-r from-primary/[0.04] to-transparent border-b border-border/60">
        <TableCell colSpan={FQDN_TABLE_COL_COUNT} className="p-0">
          <button
            type="button"
            className="w-full flex items-center justify-between px-3 py-2 text-left"
            aria-expanded={open}
            onClick={() => onOpenChange(!open)}
          >
            <div className="flex items-center gap-3">
              <SourceIcon className="size-4 text-primary/70 shrink-0" />
              <span className="font-mono text-sm font-semibold text-foreground tracking-tight">
                {group.name}
              </span>
              <span className="text-muted-foreground text-[11px] font-mono uppercase tracking-wider px-2 py-0.5 rounded-md bg-muted/60">
                {group.fqdns.length}{" "}
                {group.fqdns.length === 1 ? "entry" : "entries"}
              </span>
            </div>
            <ChevronDownIcon
              className={cn(
                "size-4 text-muted-foreground transition-transform duration-200",
                open && "rotate-180"
              )}
              aria-hidden="true"
            />
          </button>
        </TableCell>
      </TableRow>
      {open &&
        group.fqdns.map((fqdn) => <FqdnCard key={fqdn.name} fqdn={fqdn} />)}
    </>
  );
}
