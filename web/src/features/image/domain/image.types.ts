export type TagType = "semver" | "commit" | "digest" | "latest" | "other";

export type ContainerSource = "spec" | "pod";

// ChangeType mirrors the proto enum sreportal.v1.ChangeType.
export type ChangeType = "unspecified" | "none" | "mutated" | "injected";

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
  // injected: true on a pod-source ref with no matching spec ref anywhere —
  // the container was added to the pod by a MutatingWebhook (e.g. Istio
  // sidecar) and never appeared in the workload template.
  readonly injected?: boolean;
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
  // hasInjection: true when at least one of this image's workload refs is a
  // webhook-injected container (pod-only, no spec counterpart).
  readonly hasInjection?: boolean;

  // Registry version lookup fields (may be empty when not yet populated).
  readonly latestVersion?: string;
  // ISO string (from Timestamp); undefined when lookup has not run.
  readonly latestCheckedAt?: string;
  readonly latestError?: string;
  readonly upgradeAvailable?: boolean;
  readonly changeType?: ChangeType;
  // originalImage is the workload template image ref. Empty when changeType === "injected".
  readonly originalImage?: string;
  // mutatedImage is the running-pod image ref after any MutatingWebhook
  // rewrite. Equal to originalImage when changeType === "none"; canonical
  // image ref when changeType === "injected".
  readonly mutatedImage?: string;
}

export interface ImageFilters {
  readonly search: string;
  readonly registryFilter: string;
  readonly tagTypeFilter: string;
  // When true, restrict to images touched by a MutatingWebhook in the
  // matching way. mutatedFilter, injectedFilter and noneFilter combine with
  // OR — any image matching at least one enabled flag is kept. When none is
  // on, webhook activity is not used as a filter.
  readonly mutatedFilter?: boolean;
  readonly injectedFilter?: boolean;
  readonly noneFilter?: boolean;
  // Namespace multi-select filter: keep images that have at least one workload
  // in any of the selected namespaces. Empty array = no restriction.
  readonly namespaceFilter?: readonly string[];
  // changeTypeFilter: "none" | "mutated" | "injected" | "" (empty = all)
  readonly changeTypeFilter?: string;
  // upgradeFilter: when true, keep only images with upgradeAvailable === true
  readonly upgradeFilter?: boolean;
}

export interface ImageGroupStats {
  readonly total: number;
  readonly upgrades: number;
  readonly mutated: number;
  readonly injected: number;
}

export interface ImageGroup {
  readonly registry: string;
  readonly images: readonly Image[];
  readonly stats: ImageGroupStats;
}

const containerKey = (w: WorkloadRef): string =>
  `${w.kind}/${w.namespace}/${w.name}/${w.container}`;

// annotateImages enriches the API response with webhook-activity flags.
//
// Pod-source refs come in two flavors:
//   - Mutated: same workload+container appears in both a spec-source ref
//     (image A) and a pod-source ref (image B). The backend only emits the
//     pod ref when A != B, so any (spec, pod) pair for the same container is
//     a mutation. The pod ref is marked `mutated`, the matching spec ref is
//     `hidden` so the UI shows the actual running image with priority.
//   - Injected: pod-source ref with no spec counterpart anywhere — the
//     container was added by a MutatingWebhook (e.g. Istio sidecar). The ref
//     is marked `injected` so the UI can display it distinctly.
export function annotateImages(images: readonly Image[]): Image[] {
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
  const injected = new Set<string>();
  for (const [k, podLocs] of podByContainer) {
    const specLocs = specByContainer.get(k);
    if (specLocs?.length) {
      for (const l of podLocs) mutated.add(`${l.imgIdx}:${l.refIdx}`);
      for (const l of specLocs) hidden.add(`${l.imgIdx}:${l.refIdx}`);
    } else {
      for (const l of podLocs) injected.add(`${l.imgIdx}:${l.refIdx}`);
    }
  }

  return images.map((img, imgIdx) => {
    const workloads = img.workloads.map((w, refIdx) => {
      const key = `${imgIdx}:${refIdx}`;
      const isMutated = mutated.has(key);
      const isInjected = injected.has(key);
      const isHidden = hidden.has(key);
      if (!isMutated && !isInjected && !isHidden) return w;
      return {
        ...w,
        mutated: isMutated || undefined,
        injected: isInjected || undefined,
        hidden: isHidden || undefined,
      };
    });
    const hasMutation = workloads.some((w) => w.mutated);
    const hasInjection = workloads.some((w) => w.injected);
    return {
      ...img,
      workloads,
      hasMutation: hasMutation || undefined,
      hasInjection: hasInjection || undefined,
    };
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
  const webhookFilterActive = Boolean(filters.mutatedFilter || filters.injectedFilter || filters.noneFilter);
  const nsFilter = filters.namespaceFilter ?? [];
  return images.filter((img) => {
    if (filters.registryFilter && img.registry !== filters.registryFilter) return false;
    if (filters.tagTypeFilter && img.tagType !== filters.tagTypeFilter) return false;
    if (search && !img.repository.toLowerCase().includes(search)) return false;
    if (webhookFilterActive) {
      const matchesMutated = filters.mutatedFilter && img.hasMutation;
      const matchesInjected = filters.injectedFilter && img.hasInjection;
      const matchesNone = filters.noneFilter && !img.hasMutation && !img.hasInjection;
      if (!matchesMutated && !matchesInjected && !matchesNone) return false;
    }
    if (nsFilter.length > 0) {
      const imageNs = new Set(img.workloads.filter((w) => !w.hidden).map((w) => w.namespace));
      if (!nsFilter.some((ns) => imageNs.has(ns))) return false;
    }
    if (filters.changeTypeFilter) {
      if ((img.changeType ?? "unspecified") !== filters.changeTypeFilter) return false;
    }
    if (filters.upgradeFilter && !img.upgradeAvailable) return false;
    return true;
  });
}

export function computeGroupStats(images: readonly Image[]): ImageGroupStats {
  let upgrades = 0;
  let mutated = 0;
  let injected = 0;
  for (const img of images) {
    if (img.upgradeAvailable) upgrades++;
    if (img.changeType === "mutated") mutated++;
    if (img.changeType === "injected") injected++;
  }
  return { total: images.length, upgrades, mutated, injected };
}

export function groupImagesByRegistry(images: readonly Image[]): ImageGroup[] {
  const map = new Map<string, Image[]>();
  for (const image of images) {
    if (!map.has(image.registry)) map.set(image.registry, []);
    map.get(image.registry)!.push(image);
  }
  return Array.from(map.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([registry, grouped]) => {
      const sorted = [...grouped].sort((a, b) => a.repository.localeCompare(b.repository));
      return {
        registry,
        images: sorted,
        stats: computeGroupStats(sorted),
      };
    });
}
