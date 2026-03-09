export interface OriginRef {
  readonly kind: string;
  readonly namespace: string;
  readonly name: string;
}

export type SyncStatus = "sync" | "notavailable" | "notsync" | "";

export interface Fqdn {
  readonly name: string;
  readonly source: string;
  readonly groups: readonly string[];
  readonly description: string;
  readonly recordType: string;
  readonly targets: readonly string[];
  readonly dnsResourceName: string;
  readonly dnsResourceNamespace: string;
  readonly originRef?: OriginRef;
  readonly syncStatus: SyncStatus;
}

/** Returns true only when DNS resolution is confirmed in sync. */
export function isSynced(syncStatus: SyncStatus): boolean {
  return syncStatus === "sync";
}

/** Returns true when a sync status is available to display. */
export function hasSyncStatus(syncStatus: SyncStatus): boolean {
  return syncStatus !== "";
}

export interface FqdnGroup {
  readonly name: string;
  readonly source: string;
  readonly fqdns: readonly Fqdn[];
}

/** Extract unique group names from a list of FQDNs, sorted alphabetically. */
export function extractGroupNames(fqdns: readonly Fqdn[]): string[] {
  return [...new Set(fqdns.flatMap((f) => [...f.groups]))].sort();
}

/** Filter FQDNs by search term and/or group name. */
export function filterFqdns(
  fqdns: readonly Fqdn[],
  searchTerm: string,
  groupFilter: string
): Fqdn[] {
  const lowerSearch = searchTerm.toLowerCase();
  return fqdns.filter((f) => {
    const matchesSearch =
      !lowerSearch ||
      f.name.toLowerCase().includes(lowerSearch) ||
      f.description.toLowerCase().includes(lowerSearch);

    const matchesGroup = !groupFilter || f.groups.includes(groupFilter);

    return matchesSearch && matchesGroup;
  });
}

/**
 * Group filtered FQDNs by group name.
 * Each FQDN may belong to multiple groups.
 * When a group filter is active, only the matching group is included.
 */
export function groupFqdnsByGroup(
  fqdns: readonly Fqdn[],
  groupFilter: string
): FqdnGroup[] {
  const groupMap = new Map<string, { source: string; fqdns: Fqdn[] }>();

  for (const f of fqdns) {
    const targetGroups = groupFilter
      ? f.groups.filter((g) => g === groupFilter)
      : f.groups;

    for (const groupName of targetGroups) {
      if (!groupMap.has(groupName)) {
        groupMap.set(groupName, { source: f.source, fqdns: [] });
      }
      groupMap.get(groupName)!.fqdns.push(f);
    }
  }

  return Array.from(groupMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, { source, fqdns: groupFqdns }]) => ({
      name,
      source,
      fqdns: [...groupFqdns].sort((a, b) => a.name.localeCompare(b.name)),
    }));
}
