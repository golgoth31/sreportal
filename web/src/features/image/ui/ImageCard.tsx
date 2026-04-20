import { Badge } from "@/components/ui/badge";
import type { Image } from "../domain/image.types";

interface ImageCardProps {
  image: Image;
}

export function ImageCard({ image }: ImageCardProps) {
  const shortName = image.repository.split("/").at(-1) ?? image.repository;
  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-2 shadow-xs">
      <div className="flex items-center justify-between gap-2">
        <p className="font-medium text-sm">{shortName}</p>
        <Badge variant="secondary">{image.tagType}</Badge>
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
