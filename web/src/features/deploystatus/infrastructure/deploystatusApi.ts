import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  DeployStatusService,
  ListDeployStatusRequestSchema,
  type DeployStatusEntry as ProtoEntry,
  type DeployStatusCommit as ProtoCommit,
  type DeployWorkloadRef as ProtoWorkloadRef,
} from "@/gen/sreportal/v1/deploystatus_pb";
import type {
  DeployCommit,
  DeployState,
  DeployStatusEntry,
  DeployWorkloadRef,
} from "../domain/deploystatus.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(DeployStatusService, transport);

function toDomainWorkload(w: ProtoWorkloadRef): DeployWorkloadRef {
  return {
    kind: w.kind,
    namespace: w.namespace,
    name: w.name,
    container: w.container,
  };
}

function toDomainCommit(c: ProtoCommit): DeployCommit {
  return {
    sha: c.sha,
    message: c.message,
    author: c.author,
    date: c.date
      ? new Date(
          Number(c.date.seconds) * 1000 + Math.round(c.date.nanos / 1_000_000),
        ).toISOString()
      : undefined,
    url: c.url,
  };
}

function toDomainState(s: string): DeployState {
  switch (s) {
    case "ok":
      return "ok";
    case "behind":
      return "behind";
    case "unresolved":
      return "unresolved";
    default:
      return "error";
  }
}

function toDomainEntry(e: ProtoEntry): DeployStatusEntry {
  return {
    key: e.key,
    workload: e.workload ? toDomainWorkload(e.workload) : undefined,
    image: e.image,
    sourceRepo: e.sourceRepo,
    deployedRef: e.deployedRef,
    defaultBranch: e.defaultBranch,
    aheadBy: e.aheadBy,
    pendingCommits: e.pendingCommits.map(toDomainCommit),
    pendingTruncated: e.pendingTruncated,
    deployedAt: e.deployedAt
      ? new Date(
          Number(e.deployedAt.seconds) * 1000 +
            Math.round(e.deployedAt.nanos / 1_000_000),
        ).toISOString()
      : undefined,
    deployRunUrl: e.deployRunUrl,
    state: toDomainState(e.state),
    error: e.error,
    lastCheckedAt: e.lastCheckedAt
      ? new Date(
          Number(e.lastCheckedAt.seconds) * 1000 +
            Math.round(e.lastCheckedAt.nanos / 1_000_000),
        ).toISOString()
      : undefined,
  };
}

export async function listDeployStatus(
  portal: string,
): Promise<DeployStatusEntry[]> {
  const req = create(ListDeployStatusRequestSchema, {
    portal,
    search: "",
    stateFilter: "",
  });
  const res = await client.listDeployStatus(req);
  return res.entries.map(toDomainEntry);
}
