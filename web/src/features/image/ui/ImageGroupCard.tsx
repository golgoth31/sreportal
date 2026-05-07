import { ArrowUpCircleIcon, ChevronDownIcon, PackagePlusIcon, WandSparklesIcon } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import type { ImageGroup } from "../domain/image.types";
import { ImageCard } from "./ImageCard";

interface ImageGroupCardProps {
  group: ImageGroup;
}

export function ImageGroupCard({ group }: ImageGroupCardProps) {
  const [open, setOpen] = useState(true);
  const { stats } = group;
  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <div className="rounded-lg border border-border/70 bg-card/40 backdrop-blur-sm overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button
            variant="ghost"
            className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/40 bg-gradient-to-r from-primary/[0.04] to-transparent"
            aria-expanded={open}
            aria-controls={`group-${group.registry}`}
          >
            <div className="flex items-center gap-3 flex-wrap">
              <span className="font-mono text-sm font-semibold text-foreground tracking-tight">
                {group.registry}
              </span>
              <span className="text-muted-foreground text-[11px] font-mono uppercase tracking-wider px-2 py-0.5 rounded-full bg-muted/60">
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
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent id={`group-${group.registry}`}>
          <div className="border-t border-border/60 p-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {group.images.map((img) => (
              <ImageCard key={`${img.registry}/${img.repository}:${img.tag}`} image={img} />
            ))}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
