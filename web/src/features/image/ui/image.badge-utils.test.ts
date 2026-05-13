import { describe, expect, it } from "vitest";

import { tagTypeHint } from "./image.badge-utils";

describe("tagTypeHint", () => {
  it('returns "rolling tag — not tracked" for "other"', () => {
    expect(tagTypeHint("other")).toBe("rolling tag — not tracked");
  });

  it('returns "rolling tag — not tracked" for "latest"', () => {
    expect(tagTypeHint("latest")).toBe("rolling tag — not tracked");
  });

  it('returns "digest-pinned" for "digest"', () => {
    expect(tagTypeHint("digest")).toBe("digest-pinned");
  });

  it('returns "commit-pinned" for "commit"', () => {
    expect(tagTypeHint("commit")).toBe("commit-pinned");
  });

  it('returns "" for "semver"', () => {
    expect(tagTypeHint("semver")).toBe("");
  });
});
