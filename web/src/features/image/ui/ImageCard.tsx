import {
  AlertCircleIcon,
  ArrowUpCircleIcon,
  CheckIcon,
  CopyIcon,
  ExternalLinkIcon,
  LayersIcon,
  PackagePlusIcon,
  WandSparklesIcon,
} from "lucide-react";
import { useState } from "react";

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
import type { Image } from "../domain/image.types";
import {
  formatRelativeTime,
  tagTypeBadgeClass,
} from "./image.badge-utils";
import { WorkloadList } from "./WorkloadList";

interface ImageCardProps {
  image: Image;
}

const PREVIEW_LIMIT = 3;

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
  const visibleWorkloads = image.workloads.filter((w) => !w.hidden);
  const total = visibleWorkloads.length;
  const preview = visibleWorkloads.slice(0, PREVIEW_LIMIT);
  const remaining = Math.max(0, total - PREVIEW_LIMIT);
  const fullRef = `${image.registry}/${image.repository}:${image.tag}`;
  const display = `${image.repository}:${image.tag}`;

  const relativeTime = formatRelativeTime(image.latestCheckedAt);
  // Show the running-pod ref only when it actually differs from the
  // template ref (changeType === "mutated"). For "none" they're equal —
  // displaying both would be redundant. For "injected" there is no
  // template ref, so mutatedImage already represents the only known ref.
  const mutatedImage =
    image.changeType === "mutated" ? image.mutatedImage ?? fullRef : undefined;

  return (
    <>
      <div
        className={cn(
          "group rounded-lg border border-border/70 bg-card/60 backdrop-blur-sm p-4 flex flex-col gap-2 transition-all hover:border-primary/40 hover:bg-card hover:shadow-md hover:shadow-primary/5",
          image.hasMutation &&
            "border-amber-300/70 dark:border-amber-700/60 hover:border-amber-400 dark:hover:border-amber-600",
          !image.hasMutation &&
            image.hasInjection &&
            "border-violet-300/70 dark:border-violet-700/60 hover:border-violet-400 dark:hover:border-violet-600",
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 min-w-0">
            {image.upgradeAvailable && (
                  <span
                    className="inline-flex items-center text-emerald-600 dark:text-emerald-400 shrink-0"
                    aria-label="Upgrade available"
                  >
                    <ArrowUpCircleIcon className="size-3.5" />
                  </span>
            )}
            <p className="font-medium text-sm tracking-tight truncate">{shortName}</p>
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            {image.hasMutation && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className="inline-flex items-center text-amber-600 dark:text-amber-400"
                    aria-label="Image mutated by a MutatingWebhook"
                  >
                    <WandSparklesIcon className="size-3.5" />
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">Image mutated by a MutatingWebhook</p>
                </TooltipContent>
              </Tooltip>
            )}
            {image.hasInjection && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className="inline-flex items-center text-violet-600 dark:text-violet-400"
                    aria-label="Container injected by a MutatingWebhook"
                  >
                    <PackagePlusIcon className="size-3.5" />
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">Container injected by a MutatingWebhook</p>
                </TooltipContent>
              </Tooltip>
            )}
            <Badge
              variant="outline"
              className={cn(
                "font-mono uppercase text-[10px] tracking-wider",
                tagTypeBadgeClass(image.tagType),
              )}
            >
              {image.tagType}
            </Badge>
          </div>
        </div>

        <CopyableImageRef display={display} fullRef={fullRef} />

        {/* Registry version lookup row */}
        <div className="flex items-center gap-2 flex-wrap">
          {image.tagType === "semver" && (
            <span className="font-mono text-[11px] text-muted-foreground">
              {image.latestVersion ? (
                <>
                  Upgrade available:{" "}
                  <span
                    className={cn(
                      "font-semibold",
                      image.upgradeAvailable
                        ? "text-emerald-700 dark:text-emerald-400"
                        : "text-foreground/70",
                    )}
                  >
                    {image.latestVersion}
                  </span>
                </>
              ) : (
                <span className="opacity-40">latest: —</span>
              )}
            </span>
          )}
        </div>

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
            <div className="mt-2 flex items-center gap-2 flex-wrap">
              <Badge variant="outline" className={cn(tagTypeBadgeClass(image.tagType))}>
                {image.tagType}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {total} workload{total > 1 ? "s" : ""}
              </span>
            </div>
          </SheetHeader>
          <SheetBody className="mt-4 space-y-4">
            {/* Registry details section */}
            {(image.originalImage ||
              mutatedImage ||
              image.latestVersion ||
              image.latestError) && (
              <section
                className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2.5 space-y-2"
                aria-label="Registry details"
              >
                <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                  Registry details
                </p>
                {image.originalImage && (
                  <div className="flex flex-col gap-0.5">
                    <span className="text-[10px] uppercase tracking-wider text-muted-foreground/70">
                      Original image
                    </span>
                    <CopyableImageRef
                      display={image.originalImage}
                      fullRef={image.originalImage}
                    />
                  </div>
                )}
                {mutatedImage && (
                  <div className="flex flex-col gap-0.5">
                    <span className="text-[10px] uppercase tracking-wider text-muted-foreground/70">
                      Mutated image (running)
                    </span>
                    <CopyableImageRef display={mutatedImage} fullRef={mutatedImage} />
                  </div>
                )}
                {image.tagType === "semver" && (
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] uppercase tracking-wider text-muted-foreground/70">
                      Latest version
                    </span>
                    <span
                      className={cn(
                        "font-mono text-xs",
                        image.upgradeAvailable
                          ? "text-emerald-700 dark:text-emerald-400 font-semibold"
                          : "text-muted-foreground",
                      )}
                    >
                      {image.latestVersion ?? "—"}
                    </span>
                    {image.upgradeAvailable && (
                      <ArrowUpCircleIcon
                        className="size-3.5 text-emerald-600 dark:text-emerald-400"
                        aria-label="Upgrade available"
                      />
                    )}
                  </div>
                )}
                {relativeTime && (
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] uppercase tracking-wider text-muted-foreground/70">
                      Last checked
                    </span>
                    <span className="font-mono text-xs text-muted-foreground">{relativeTime}</span>
                  </div>
                )}
                {image.latestError && (
                  <div
                    className="flex items-start gap-1.5 rounded bg-destructive/10 border border-destructive/20 px-2 py-1.5"
                    role="alert"
                  >
                    <AlertCircleIcon className="size-3.5 mt-0.5 shrink-0 text-destructive" aria-hidden="true" />
                    <p className="font-mono text-[11px] text-destructive break-all">
                      {image.latestError}
                    </p>
                  </div>
                )}
              </section>
            )}
            <WorkloadList workloads={visibleWorkloads} variant="full" />
          </SheetBody>
        </SheetContent>
      </Sheet>
    </>
  );
}
