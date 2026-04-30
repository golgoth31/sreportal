import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { Image, TagType } from "../domain/image.types";

interface ImageCardProps {
  image: Image;
}

export function tagTypeBadgeClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/30 dark:text-green-300",
    commit:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    digest:
      "border-purple-200 bg-purple-100 text-purple-800 dark:border-purple-800 dark:bg-purple-900/30 dark:text-purple-300",
    latest:
      "border-amber-200 bg-amber-100 text-amber-800 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-300",
  };
  return classes[tagType];
}

export function ImageCard({ image }: ImageCardProps) {
  const shortName = image.repository.split("/").at(-1) ?? image.repository;
  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-2 shadow-xs">
      <div className="flex items-center justify-between gap-2">
        <p className="font-medium text-sm">{shortName}</p>
        <Badge variant="outline" className={cn(tagTypeBadgeClass(image.tagType))}>
          {image.tagType}
        </Badge>
      </div>
      <p className="text-xs text-muted-foreground font-mono break-all">
        {image.repository}:{image.tag}
      </p>
      <p className="text-xs text-muted-foreground">
        {image.workloads.length} workload{image.workloads.length > 1 ? "s" : ""}
      </p>
    </div>
  );
}
