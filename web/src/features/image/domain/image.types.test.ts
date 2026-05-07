import { describe, expect, it } from "vitest";

import {
  annotateImages,
  computeGroupStats,
  filterImages,
  groupImagesByRegistry,
  hasVisibleWorkloads,
  type Image,
} from "./image.types";
import { formatRelativeTime, tagTypeBadgeClass } from "../ui/image.badge-utils";

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

describe("filterImages — webhook filters", () => {
  const dataset: Image[] = [
    {
      registry: "ghcr.io",
      repository: "acme/api",
      tag: "1.0.0",
      tagType: "semver",
      workloads: [],
    },
    {
      registry: "ghcr.io",
      repository: "acme/api",
      tag: "1.0.1-pinned",
      tagType: "semver",
      workloads: [],
      hasMutation: true,
    },
    {
      registry: "docker.io",
      repository: "istio/proxyv2",
      tag: "1.20.0",
      tagType: "semver",
      workloads: [],
      hasInjection: true,
    },
  ];

  it("returns everything when no webhook filter is active", () => {
    const out = filterImages(dataset, { search: "", registryFilter: "", tagTypeFilter: "" });
    expect(out).toHaveLength(3);
  });

  it("keeps only mutated images when mutatedFilter is on", () => {
    const out = filterImages(dataset, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      mutatedFilter: true,
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.tag).toBe("1.0.1-pinned");
  });

  it("keeps only injected images when injectedFilter is on", () => {
    const out = filterImages(dataset, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      injectedFilter: true,
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("istio/proxyv2");
  });

  it("ORs mutated and injected when both filters are on", () => {
    const out = filterImages(dataset, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      mutatedFilter: true,
      injectedFilter: true,
    });
    expect(out).toHaveLength(2);
    expect(out.map((i) => i.repository).sort()).toEqual(["acme/api", "istio/proxyv2"]);
  });

  it("combines webhook filters with search/registry/tagType", () => {
    const out = filterImages(dataset, {
      search: "",
      registryFilter: "ghcr.io",
      tagTypeFilter: "",
      mutatedFilter: true,
      injectedFilter: true,
    });
    // Only the mutated ghcr.io entry remains; the injected one is on docker.io.
    expect(out).toHaveLength(1);
    expect(out[0]?.tag).toBe("1.0.1-pinned");
  });
});

describe("annotateImages", () => {
  it("does nothing when changeType is none or unset", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.0",
        tagType: "semver",
        changeType: "none",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
      {
        registry: "ghcr.io",
        repository: "acme/web",
        tag: "1.0.0",
        tagType: "semver",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "web", container: "main", source: "spec" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.hasMutation).toBeUndefined();
    expect(out[0]?.hasInjection).toBeUndefined();
    expect(out[0]?.workloads[0]?.mutated).toBeUndefined();
    expect(out[1]?.hasMutation).toBeUndefined();
  });

  it("flags hasMutation and tags every workload when changeType is mutated", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        changeType: "mutated",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.hasMutation).toBe(true);
    expect(out[0]?.hasInjection).toBeUndefined();
    expect(out[0]?.workloads[0]?.mutated).toBe(true);
    expect(out[0]?.workloads[0]?.injected).toBeUndefined();
  });

  it("flags hasInjection and tags every workload when changeType is injected", () => {
    const input: Image[] = [
      {
        registry: "docker.io",
        repository: "istio/proxyv2",
        tag: "1.20.0",
        tagType: "semver",
        changeType: "injected",
        workloads: [
          {
            kind: "Deployment",
            namespace: "default",
            name: "api",
            container: "istio-proxy",
            source: "spec",
          },
        ],
      },
    ];
    const out = annotateImages(input);
    expect(out[0]?.hasInjection).toBe(true);
    expect(out[0]?.hasMutation).toBeUndefined();
    expect(out[0]?.workloads[0]?.injected).toBe(true);
    expect(out[0]?.workloads[0]?.mutated).toBeUndefined();
  });

  it("hasVisibleWorkloads keeps images that have any non-hidden workload", () => {
    const input: Image[] = [
      {
        registry: "ghcr.io",
        repository: "acme/api",
        tag: "1.0.1-pinned",
        tagType: "semver",
        changeType: "mutated",
        workloads: [
          { kind: "Deployment", namespace: "default", name: "api", container: "web", source: "spec" },
        ],
      },
    ];
    const out = annotateImages(input).filter(hasVisibleWorkloads);
    expect(out).toHaveLength(1);
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

// -------------------------------------------------------------------
// New filter fields: namespace, changeType, upgradeAvailable
// -------------------------------------------------------------------

const enriched: Image[] = [
  {
    registry: "ghcr.io",
    repository: "acme/api",
    tag: "1.0.0",
    tagType: "semver",
    changeType: "none",
    upgradeAvailable: true,
    latestVersion: "1.1.0",
    workloads: [
      { kind: "Deployment", namespace: "prod", name: "api", container: "main", source: "spec" },
    ],
  },
  {
    registry: "ghcr.io",
    repository: "acme/web",
    tag: "1.0.0",
    tagType: "semver",
    changeType: "mutated",
    upgradeAvailable: false,
    workloads: [
      { kind: "Deployment", namespace: "staging", name: "web", container: "main", source: "pod" },
    ],
  },
  {
    registry: "docker.io",
    repository: "istio/proxyv2",
    tag: "1.20.0",
    tagType: "semver",
    changeType: "injected",
    upgradeAvailable: false,
    workloads: [
      { kind: "Deployment", namespace: "prod", name: "api", container: "istio-proxy", source: "pod" },
    ],
  },
];

describe("filterImages — new fields", () => {
  it("filters by upgradeFilter", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      upgradeFilter: true,
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("acme/api");
  });

  it("filters by changeTypeFilter", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      changeTypeFilter: "injected",
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("istio/proxyv2");
  });

  it("filters by namespace (multi-select, OR)", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      namespaceFilter: ["staging"],
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("acme/web");
  });

  it("namespace filter with two values returns images in either ns", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      namespaceFilter: ["prod", "staging"],
    });
    // acme/api (prod) + acme/web (staging) + istio/proxyv2 (prod)
    expect(out).toHaveLength(3);
  });

  it("empty namespaceFilter returns all images", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      namespaceFilter: [],
    });
    expect(out).toHaveLength(3);
  });

  it("combines changeTypeFilter and upgradeFilter", () => {
    const out = filterImages(enriched, {
      search: "",
      registryFilter: "",
      tagTypeFilter: "",
      changeTypeFilter: "none",
      upgradeFilter: true,
    });
    expect(out).toHaveLength(1);
    expect(out[0]?.repository).toBe("acme/api");
  });
});

describe("computeGroupStats", () => {
  it("counts upgrades, mutated, and injected images correctly", () => {
    const stats = computeGroupStats(enriched);
    expect(stats.total).toBe(3);
    expect(stats.upgrades).toBe(1);
    expect(stats.mutated).toBe(1);
    expect(stats.injected).toBe(1);
  });

  it("returns zeros for plain images", () => {
    const stats = computeGroupStats(base);
    expect(stats.total).toBe(3);
    expect(stats.upgrades).toBe(0);
    expect(stats.mutated).toBe(0);
    expect(stats.injected).toBe(0);
  });
});

describe("groupImagesByRegistry — stats", () => {
  it("includes stats on each group", () => {
    const groups = groupImagesByRegistry(enriched);
    const ghcr = groups.find((g) => g.registry === "ghcr.io");
    expect(ghcr?.stats.total).toBe(2);
    expect(ghcr?.stats.upgrades).toBe(1);
    expect(ghcr?.stats.mutated).toBe(1);
  });
});

describe("formatRelativeTime", () => {
  it("returns null for undefined", () => {
    expect(formatRelativeTime(undefined)).toBeNull();
  });

  it("returns a relative string for a valid ISO date", () => {
    const recent = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(); // 2h ago
    const result = formatRelativeTime(recent);
    expect(result).not.toBeNull();
    expect(result).toContain("ago");
  });

  it("returns null for an invalid date string", () => {
    expect(formatRelativeTime("not-a-date")).toBeNull();
  });
});
