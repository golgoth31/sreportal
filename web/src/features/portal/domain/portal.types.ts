export interface RemoteSyncStatus {
  readonly lastSyncTime: string;
  readonly lastSyncError: string;
  readonly remoteTitle: string;
  readonly fqdnCount: number;
}

export interface Portal {
  readonly name: string;
  readonly title: string;
  readonly main: boolean;
  readonly subPath: string;
  readonly namespace: string;
  readonly ready: boolean;
  readonly url: string;
  readonly isRemote: boolean;
  readonly remoteSync?: RemoteSyncStatus;
}

/** True when the controller reported a non-empty last sync error (stale remote data). */
export function hasRemoteSyncError(portal: Portal | undefined): boolean {
  const err = portal?.remoteSync?.lastSyncError?.trim();
  return Boolean(err);
}
