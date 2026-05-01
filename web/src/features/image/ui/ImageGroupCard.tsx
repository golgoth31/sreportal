import { ChevronDownIcon } from "lucide-react";
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
  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <div className="rounded-lg border border-border/70 bg-card/40 backdrop-blur-sm overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button variant="ghost" className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/40 bg-gradient-to-r from-primary/[0.04] to-transparent">
            <div className="flex items-center gap-3">
              <span className="font-mono text-sm font-semibold text-foreground tracking-tight">{group.registry}</span>
              <span className="text-muted-foreground text-[11px] font-mono uppercase tracking-wider px-2 py-0.5 rounded-full bg-muted/60">
                {group.images.length} {group.images.length === 1 ? "image" : "images"}
              </span>
            </div>
            <ChevronDownIcon className={cn("size-4 text-muted-foreground transition-transform duration-200", open && "rotate-180")} />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
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
