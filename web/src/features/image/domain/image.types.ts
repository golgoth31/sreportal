export type TagType = "semver" | "commit" | "digest" | "latest" | "other";

export type ContainerSource = "spec" | "pod";

export interface WorkloadRef {
  readonly kind: string;
  readonly namespace: string;
  readonly name: string;
  readonly container: string;
  readonly source: ContainerSource;
  // mutated: true on a pod-source ref that has a matching spec ref in another
  // image (same workload+container, different image). The card displaying this
  // ref is the "actual running" image after webhook mutation.
  readonly mutated?: boolean;
  // hidden: true on a spec-source ref whose image was replaced by a webhook
  // mutation. UI omits these from listings; the matching pod-source ref takes
  // priority.
  readonly hidden?: boolean;
}

export interface Image {
  readonly registry: string;
  readonly repository: string;
  readonly tag: string;
  readonly tagType: TagType;
  readonly workloads: readonly WorkloadRef[];
  // hasMutation: true when at least one of this image's workload refs is the
  // mutated (pod-source) side of a webhook mutation.
  readonly hasMutation?: boolean;
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

const containerKey = (w: WorkloadRef): string =>
  `${w.kind}/${w.namespace}/${w.name}/${w.container}`;

// annotateMutations enriches the API response with mutation/hidden flags.
//
// Mutation detection: when the same workload+container appears in both a
// spec-source ref (image A) and a pod-source ref (image B), the backend only
// emits the pod ref when A != B — so any (spec, pod) pair for the same
// container *is* a mutation. The pod ref is marked mutated, the spec ref
// hidden so that the UI displays the actual running image with priority.
//
// Pod-source refs without any matching spec ref are injected sidecars; they
// stay visible and unmarked.
export function annotateMutations(images: readonly Image[]): Image[] {
  const specByContainer = new Map<string, Array<{ imgIdx: number; refIdx: number }>>();
  const podByContainer = new Map<string, Array<{ imgIdx: number; refIdx: number }>>();

  images.forEach((img, imgIdx) => {
    img.workloads.forEach((w, refIdx) => {
      const k = containerKey(w);
      const bucket = w.source === "pod" ? podByContainer : specByContainer;
      const list = bucket.get(k);
      if (list) list.push({ imgIdx, refIdx });
      else bucket.set(k, [{ imgIdx, refIdx }]);
    });
  });

  const mutated = new Set<string>();
  const hidden = new Set<string>();
  for (const [k, podLocs] of podByContainer) {
    const specLocs = specByContainer.get(k);
    if (!specLocs?.length) continue;
    for (const l of podLocs) mutated.add(`${l.imgIdx}:${l.refIdx}`);
    for (const l of specLocs) hidden.add(`${l.imgIdx}:${l.refIdx}`);
  }

  return images.map((img, imgIdx) => {
    const workloads = img.workloads.map((w, refIdx) => {
      const key = `${imgIdx}:${refIdx}`;
      const isMutated = mutated.has(key);
      const isHidden = hidden.has(key);
      if (!isMutated && !isHidden) return w;
      return { ...w, mutated: isMutated || undefined, hidden: isHidden || undefined };
    });
    const hasMutation = workloads.some((w) => w.mutated);
    return hasMutation === Boolean(img.hasMutation) && workloads === img.workloads
      ? img
      : { ...img, workloads, hasMutation: hasMutation || undefined };
  });
}

// hasVisibleWorkloads returns true when the image still has at least one
// workload ref that isn't hidden by a mutation. Images whose every ref was
// shadowed by mutations should be dropped from the listing.
export function hasVisibleWorkloads(img: Image): boolean {
  return img.workloads.some((w) => !w.hidden);
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
