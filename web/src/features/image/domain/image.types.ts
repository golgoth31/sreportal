export type TagType = "semver" | "commit" | "digest" | "latest" | "other";

export interface WorkloadRef {
  readonly kind: string;
  readonly namespace: string;
  readonly name: string;
  readonly container: string;
}

export interface Image {
  readonly registry: string;
  readonly repository: string;
  readonly tag: string;
  readonly tagType: TagType;
  readonly workloads: readonly WorkloadRef[];
}

export interface ImageFilters {
  readonly search: string;
  readonly registryFilter: string;
  readonly tagTypeFilter: string;
}

export interface ImageGroup {
  readonly registry: string;
  readonly images: readonly Image[];
}

export function filterImages(images: readonly Image[], filters: ImageFilters): Image[] {
  const search = filters.search.toLowerCase();
  return images.filter((img) => {
    if (filters.registryFilter && img.registry !== filters.registryFilter) return false;
    if (filters.tagTypeFilter && img.tagType !== filters.tagTypeFilter) return false;
    if (search && !img.repository.toLowerCase().includes(search)) return false;
    return true;
  });
}

export function groupImagesByRegistry(images: readonly Image[]): ImageGroup[] {
  const map = new Map<string, Image[]>();
  for (const image of images) {
    if (!map.has(image.registry)) map.set(image.registry, []);
    map.get(image.registry)!.push(image);
  }
  return Array.from(map.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([registry, grouped]) => ({
      registry,
      images: [...grouped].sort((a, b) => a.repository.localeCompare(b.repository)),
    }));
}
