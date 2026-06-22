export type DeployState = "ok" | "behind" | "unresolved" | "error";

export interface DeployWorkloadRef {
  readonly kind: string;
  readonly namespace: string;
  readonly name: string;
  readonly container: string;
}

export interface DeployCommit {
  readonly sha: string;
  readonly message: string;
  readonly author: string;
  readonly date?: string;
  readonly url: string;
}

export interface DeployStatusEntry {
  readonly key: string;
  readonly workload?: DeployWorkloadRef;
  readonly image: string;
  readonly sourceRepo: string;
  readonly deployedRef: string;
  readonly defaultBranch: string;
  readonly aheadBy: number;
  readonly pendingCommits: DeployCommit[];
  readonly pendingTruncated: boolean;
  readonly deployedAt?: string;
  readonly deployRunUrl: string;
  readonly state: DeployState;
  readonly error: string;
  readonly lastCheckedAt?: string;
}
