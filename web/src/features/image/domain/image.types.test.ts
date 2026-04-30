import { describe, expect, it } from "vitest";

import { filterImages, groupImagesByRegistry, type Image } from "./image.types";
import { tagTypeBadgeClass } from "../ui/ImageCard";

const base: Image[] = [
  { registry: "ghcr.io", repository: "acme/api", tag: "1.0.0", tagType: "semver", workloads: [] },
  { registry: "ghcr.io", repository: "acme/web", tag: "latest", tagType: "latest", workloads: [] },
  { registry: "docker.io", repository: "library/nginx", tag: "sha256:abc", tagType: "digest", workloads: [] },
];

describe("image.types", () => {
  it("filterImages applique les filtres", () => {
    const out = filterImages(base, { search: "acme/", registryFilter: "ghcr.io", tagTypeFilter: "semver" });
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
