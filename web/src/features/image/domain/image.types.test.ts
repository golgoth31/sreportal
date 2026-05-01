import { describe, expect, it } from "vitest";

import {
  annotateImages,
  filterImages,
  groupImagesByRegistry,
  hasVisibleWorkloads,
  type Image,
} from "./image.types";
import { tagTypeBadgeClass } from "../ui/ImageCard";

const base: Image[] = [
  {
    registry: "ghcr.io",
    repository: "acme/api",
    tag: "1.0.0",
    tagType: "semver",
    workloads: [],
  },
  {
    registry: "ghcr.io",
    repository: "acme/web",
    tag: "latest",
    tagType: "latest",
    workloads: [],
  },
  {
    registry: "docker.io",
    repository: "library/nginx",
    tag: "sha256:abc",
    tagType: "digest",
    workloads: [],
  },
];

describe("image.types", () => {
  it("filterImages applique les filtres", () => {
    const out = filterImages(base, {
      search: "acme/",
      registryFilter: "ghcr.io",
      tagTypeFilter: "semver",
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("acme/api");
  });

  it("groupImagesByRegistry groupe par registre", () => {
    const out = groupImagesByRegistry(base);
    expect(out).toHaveLength(2);
    expect(out[0]?.registry).toBe("docker.io");
    expect(out[1]?.registry).toBe("ghcr.io");
  });
});

describe("annotateImages", () => {
  it("does nothing when there are only spec workloads", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.workloads[0]?.mutated).toBeUndefined();
    expect(out[0]?.workloads[0]?.hidden).toBeUndefined();
    expect(out[0]?.hasMutation).toBeUndefined();
  });

  it("marks a pod ref as mutated and hides the matching spec ref", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "pod" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.workloads[0]?.hidden).toBe(true);
    expect(out[1]?.workloads[0]?.mutated).toBe(true);
    expect(out[1]?.hasMutation).toBe(true);
    expect(out[0]?.hasMutation).toBeUndefined();
  });

  it("marks injected sidecars (pod-only) as injected, not mutated", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
      {
        registry: "docker.io",
        repository: "istio/proxyv2",
        tag: "1.20.0",
        tagType: "semver",
        workloads: [
          {
            kind: "Deployment",
            namespace: "default",
            name: "api",
            container: "istio-proxy",
            source: "pod",
          },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[1]?.workloads[0]?.mutated).toBeUndefined();
    expect(out[1]?.workloads[0]?.injected).toBe(true);
    expect(out[1]?.hasMutation).toBeUndefined();
    expect(out[1]?.hasInjection).toBe(true);
  });

  it("does not mark spec refs as injected", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.workloads[0]?.injected).toBeUndefined();
    expect(out[0]?.hasInjection).toBeUndefined();
  });

  it("does not mark mutated pod refs as injected", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "pod" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[1]?.workloads[0]?.mutated).toBe(true);
    expect(out[1]?.workloads[0]?.injected).toBeUndefined();
    expect(out[1]?.hasInjection).toBeUndefined();
  });

  it("hasVisibleWorkloads drops images whose every ref is hidden", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "pod" },
        ],
      },
    ];
    const out = annotateImages(input).filter(hasVisibleWorkloads);
    expect(out).toHaveLength(1);
    expect(out[0]?.tag).toBe("1.0.1-pinned");
  });

  it("preserves visible spec refs for unmutated containers on the same image", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
          { kind: "Deployment", namespace: "default", name: "api", container: "worker", source: "spec" },
        ],
      },
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "pod" },
        ],
      },
    ];
    const out = annotateImages(input).filter(hasVisibleWorkloads);
    // Spec image still visible thanks to "worker" ref; "web" hidden.
    expect(out).toHaveLength(2);
    const spec = out.find((i) => i.tag === "1.0.0");
    expect(spec?.workloads.find((w) => w.container === "web")?.hidden).toBe(true);
    expect(spec?.workloads.find((w) => w.container === "worker")?.hidden).toBeUndefined();
  });
});

describe("tagTypeBadgeClass", () => {
  it("returns blue classes for semver", () => {
    expect(tagTypeBadgeClass("semver")).toContain("blue");
  });

  it("returns blue classes for commit", () => {
    expect(tagTypeBadgeClass("commit")).toContain("blue");
  });

  it("returns green classes for digest", () => {
    expect(tagTypeBadgeClass("digest")).toContain("green");
  });

  it("returns red classes for latest", () => {
    expect(tagTypeBadgeClass("latest")).toContain("red");
  });

  it("returns gray classes for other", () => {
    expect(tagTypeBadgeClass("other")).toContain("gray");
  });
});
