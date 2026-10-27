import {
  ArrowUpCircleIcon,
  ChevronDownIcon,
  PackagePlusIcon,
  WandSparklesIcon,
} from "lucide-react";

import { TableCell, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type { ImageGroup } from "../domain/image.types";
import { ImageCard } from "./ImageCard";

export const IMAGE_TABLE_COL_COUNT = 7;

interface ImageGroupCardProps {
  group: ImageGroup;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Group header + data rows for a parent page-level Soft Material table. */
export function ImageGroupCard({ group, open, onOpenChange }: ImageGroupCardProps) {
  const { stats } = group;

  return (
    <>
      <TableRow className="hover:bg-muted/40 bg-gradient-to-r from-primary/[0.04] to-transparent border-b border-border/60">
        <TableCell colSpan={IMAGE_TABLE_COL_COUNT} className="p-0">
          <button
            type="button"
            className="w-full flex items-center justify-between px-3 py-2 text-left"
            aria-expanded={open}
            aria-controls={`group-${group.registry}`}
            onClick={() => onOpenChange(!open)}
          >
            <div className="flex items-center gap-3 flex-wrap">
              <span className="font-mono text-sm font-semibold text-foreground tracking-tight">
                {group.registry}
              </span>
              <span className="text-muted-foreground text-[11px] font-mono uppercase tracking-wider px-2 py-0.5 rounded-md bg-muted/60">
                {stats.total} {stats.total === 1 ? "image" : "images"}
              </span>
              {stats.upgrades > 0 && (
                <span
                  className="inline-flex items-center gap-1 text-[11px] font-mono text-emerald-700 dark:text-emerald-400"
                  aria-label={`${stats.upgrades} upgrade${stats.upgrades > 1 ? "s" : ""} available`}
                >
                  <ArrowUpCircleIcon className="size-3" aria-hidden="true" />
                  {stats.upgrades}
                </span>
              )}
              {stats.mutated > 0 && (
                <span
                  className="inline-flex items-center gap-1 text-[11px] font-mono text-amber-700 dark:text-amber-400"
                  aria-label={`${stats.mutated} mutated`}
                >
                  <WandSparklesIcon className="size-3" aria-hidden="true" />
                  {stats.mutated}
                </span>
              )}
              {stats.injected > 0 && (
                <span
                  className="inline-flex items-center gap-1 text-[11px] font-mono text-violet-700 dark:text-violet-400"
                  aria-label={`${stats.injected} injected`}
                >
                  <PackagePlusIcon className="size-3" aria-hidden="true" />
                  {stats.injected}
                </span>
              )}
            </div>
            <ChevronDownIcon
              className={cn(
                "size-4 text-muted-foreground transition-transform duration-200",
                open && "rotate-180",
              )}
              aria-hidden="true"
            />
          </button>
        </TableCell>
      </TableRow>
      {open &&
        group.images.map((img) => (
          <ImageCard
            key={`${img.registry}/${img.repository}:${img.tag}`}
            image={img}
          />
        ))}
    </>
  );
}
