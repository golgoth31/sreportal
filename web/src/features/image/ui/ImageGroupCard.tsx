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
      <div className="rounded-lg border bg-card shadow-xs overflow-hidden">
        <CollapsibleTrigger asChild>
          <Button variant="ghost" className="w-full flex items-center justify-between px-4 py-3 h-auto rounded-none hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <span className="font-semibold text-sm">{group.registry}</span>
              <span className="text-muted-foreground text-xs">{group.images.length} images</span>
            </div>
            <ChevronDownIcon className={cn("size-4 text-muted-foreground transition-transform duration-200", open && "rotate-180")} />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="border-t p-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {group.images.map((img) => (
              <ImageCard key={`${img.registry}/${img.repository}:${img.tag}`} image={img} />
            ))}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
