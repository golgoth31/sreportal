import { useState } from "react";
import { CheckIcon, CopyIcon, ExternalLinkIcon, LayersIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { cn } from "@/lib/utils";
import type { Image, TagType } from "../domain/image.types";
import { WorkloadList } from "./WorkloadList";

interface ImageCardProps {
  image: Image;
}

const PREVIEW_LIMIT = 3;

export function tagTypeBadgeClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    commit:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    digest:
      "border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/30 dark:text-green-300",
    latest:
      "border-red-200 bg-red-100 text-red-800 dark:border-red-800 dark:bg-red-900/30 dark:text-red-300",
    other:
      "border-gray-200 bg-gray-100 text-gray-700 dark:border-gray-700 dark:bg-gray-800/40 dark:text-gray-300",
  };
  return classes[tagType];
}

export function tagTypeBadgeMutedClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-blue-200/70 bg-transparent text-blue-700/70 hover:bg-blue-50 hover:text-blue-800 dark:border-blue-800/50 dark:text-blue-400/70 dark:hover:bg-blue-900/20 dark:hover:text-blue-300",
    commit:
      "border-blue-200/70 bg-transparent text-blue-700/70 hover:bg-blue-50 hover:text-blue-800 dark:border-blue-800/50 dark:text-blue-400/70 dark:hover:bg-blue-900/20 dark:hover:text-blue-300",
    digest:
      "border-green-200/70 bg-transparent text-green-700/70 hover:bg-green-50 hover:text-green-800 dark:border-green-800/50 dark:text-green-400/70 dark:hover:bg-green-900/20 dark:hover:text-green-300",
    latest:
      "border-red-200/70 bg-transparent text-red-700/70 hover:bg-red-50 hover:text-red-800 dark:border-red-800/50 dark:text-red-400/70 dark:hover:bg-red-900/20 dark:hover:text-red-300",
    other:
      "border-gray-200/70 bg-transparent text-gray-600/70 hover:bg-gray-50 hover:text-gray-800 dark:border-gray-700/50 dark:text-gray-400/70 dark:hover:bg-gray-800/30 dark:hover:text-gray-200",
  };
  return classes[tagType];
}

interface CopyableImageRefProps {
  display: string;
  fullRef: string;
  className?: string;
}

function CopyableImageRef({ display, fullRef, className }: CopyableImageRefProps) {
  const { copied, copy } = useCopyToClipboard(fullRef);
  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        "group/copy -mx-1.5 inline-flex w-fit items-start gap-1 rounded-md border border-transparent px-1.5 py-0.5",
        "text-left text-xs text-muted-foreground font-mono break-all transition-colors",
        "hover:border-border hover:bg-muted/60 hover:text-foreground",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
      aria-label={`Copy ${fullRef} to clipboard`}
    >
      <span className="flex-1">{display}</span>
      {copied ? (
        <CheckIcon className="mt-0.5 size-3 shrink-0 text-green-600" />
      ) : (
        <CopyIcon className="mt-0.5 size-3 shrink-0 opacity-0 transition-opacity group-hover/copy:opacity-70" />
      )}
    </button>
  );
}

export function ImageCard({ image }: ImageCardProps) {
  const [open, setOpen] = useState(false);
  const shortName = image.repository.split("/").at(-1) ?? image.repository;
  const total = image.workloads.length;
  const preview = image.workloads.slice(0, PREVIEW_LIMIT);
  const remaining = Math.max(0, total - PREVIEW_LIMIT);
  const fullRef = `${image.registry}/${image.repository}:${image.tag}`;
  const display = `${image.repository}:${image.tag}`;

  return (
    <>
      <div className="group rounded-lg border border-border/70 bg-card/60 backdrop-blur-sm p-4 flex flex-col gap-2 transition-all hover:border-primary/40 hover:bg-card hover:shadow-md hover:shadow-primary/5">
        <div className="flex items-center justify-between gap-2">
          <p className="font-medium text-sm tracking-tight">{shortName}</p>
          <Badge variant="outline" className={cn("font-mono uppercase text-[10px] tracking-wider", tagTypeBadgeClass(image.tagType))}>
            {image.tagType}
          </Badge>
        </div>
        <CopyableImageRef display={display} fullRef={fullRef} />

        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={() => setOpen(true)}
              className={cn(
                "group/wl mt-1 inline-flex w-fit items-center gap-1.5 rounded-md border border-transparent px-1.5 py-0.5 -mx-1.5",
                "text-xs text-muted-foreground transition-colors",
                "hover:border-border hover:bg-muted/60 hover:text-foreground",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              )}
              aria-label={`View ${total} workload${total > 1 ? "s" : ""} using this image`}
            >
              <LayersIcon className="size-3" />
              <span>
                {total} workload{total > 1 ? "s" : ""}
              </span>
              <ExternalLinkIcon className="size-2.5 opacity-0 transition-opacity group-hover/wl:opacity-70" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="top" align="start">
            <div className="flex flex-col gap-1.5">
              <p className="text-[10px] font-semibold uppercase tracking-wider opacity-70">
                Source resources
              </p>
              <WorkloadList workloads={preview} variant="compact" />
              {remaining > 0 && (
                <p className="text-[11px] italic opacity-70">
                  +{remaining} more — click to view all
                </p>
              )}
            </div>
          </TooltipContent>
        </Tooltip>
      </div>

      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle className="flex items-center gap-2">
              <LayersIcon className="size-4 text-muted-foreground" />
              Source resources
            </SheetTitle>
            <SheetDescription asChild>
              <CopyableImageRef display={display} fullRef={fullRef} />
            </SheetDescription>
            <div className="mt-2 flex items-center gap-2">
              <Badge variant="outline" className={cn(tagTypeBadgeClass(image.tagType))}>
                {image.tagType}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {total} workload{total > 1 ? "s" : ""}
              </span>
            </div>
          </SheetHeader>
          <SheetBody className="mt-4">
            <WorkloadList workloads={image.workloads} variant="full" />
          </SheetBody>
        </SheetContent>
      </Sheet>
    </>
  );
}
