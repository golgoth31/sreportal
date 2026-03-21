import { describe, expect, it } from "vitest";

import {
  extractGroupNames,
  filterFqdns,
  groupFqdnsByGroup,
  hasSyncStatus,
  isSynced,
  type Fqdn,
} from "./dns.types";

function fqdn(overrides: Partial<Fqdn> & Pick<Fqdn, "name">): Fqdn {
  return {
    name: overrides.name,
    source: overrides.source ?? "external-dns",
    groups: overrides.groups ?? [],
    description: overrides.description ?? "",
    recordType: overrides.recordType ?? "A",
    targets: overrides.targets ?? [],
    dnsResourceName: overrides.dnsResourceName ?? "dns",
    dnsResourceNamespace: overrides.dnsResourceNamespace ?? "default",
    originRef: overrides.originRef,
    syncStatus: overrides.syncStatus ?? "",
  };
}

describe("isSynced", () => {
  it("returns true only for sync status", () => {
    expect(isSynced("sync")).toBe(true);
    expect(isSynced("notsync")).toBe(false);
    expect(isSynced("notavailable")).toBe(false);
    expect(isSynced("")).toBe(false);
  });
});

describe("hasSyncStatus", () => {
  it("returns false for empty string and true otherwise", () => {
    expect(hasSyncStatus("")).toBe(false);
    expect(hasSyncStatus("sync")).toBe(true);
  });
});

describe("extractGroupNames", () => {
  it("returns unique group names sorted alphabetically", () => {
    const fqdns = [
      fqdn({ name: "a.example.com", groups: ["z", "a"] }),
      fqdn({ name: "b.example.com", groups: ["a", "m"] }),
    ];
    expect(extractGroupNames(fqdns)).toEqual(["a", "m", "z"]);
  });
});

describe("filterFqdns", () => {
  const items = [
    fqdn({
      name: "api.prod.example.com",
      description: "API",
      groups: ["prod"],
    }),
    fqdn({
      name: "cache.prod.example.com",
      description: "Redis",
      groups: ["prod"],
    }),
  ];

  it("matches search on name case-insensitively", () => {
    const out = filterFqdns(items, "API", "");
    expect(out.map((f) => f.name)).toEqual(["api.prod.example.com"]);
  });

  it("matches search on description", () => {
    const out = filterFqdns(items, "redis", "");
    expect(out.map((f) => f.name)).toEqual(["cache.prod.example.com"]);
  });

  it("filters by group when groupFilter is set", () => {
    const out = filterFqdns(items, "", "prod");
    expect(out).toHaveLength(2);
  });

  it("returns empty when group does not match", () => {
    const out = filterFqdns(items, "", "staging");
    expect(out).toHaveLength(0);
  });
});

describe("groupFqdnsByGroup", () => {
  it("places each FQDN under each of its groups and sorts groups and names", () => {
    const items = [
      fqdn({ name: "b.example.com", groups: ["g2", "g1"], source: "s" }),
      fqdn({ name: "a.example.com", groups: ["g1"], source: "s" }),
    ];
    const groups = groupFqdnsByGroup(items, "");
    expect(groups.map((g) => g.name)).toEqual(["g1", "g2"]);
    const g1 = groups.find((g) => g.name === "g1");
    expect(g1?.fqdns.map((f) => f.name)).toEqual([
      "a.example.com",
      "b.example.com",
    ]);
  });

  it("when group filter is active only includes that group", () => {
    const items = [
      fqdn({ name: "x.example.com", groups: ["a", "b"], source: "s" }),
    ];
    const groups = groupFqdnsByGroup(items, "b");
    expect(groups.map((g) => g.name)).toEqual(["b"]);
    expect(groups[0]?.fqdns).toHaveLength(1);
  });
});
