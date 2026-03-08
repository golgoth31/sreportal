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
