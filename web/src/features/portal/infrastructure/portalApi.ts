import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import {
  ListPortalsRequestSchema,
  type Portal as ProtoPortal,
  PortalService,
} from "@/gen/sreportal/v1/portal_pb";
import type { Portal } from "../domain/portal.types";

function createTransport() {
  return createConnectTransport({ baseUrl: window.location.origin });
}

function toDomainPortal(p: ProtoPortal): Portal {
  return {
    name: p.name,
    title: p.title,
    main: p.main,
    subPath: p.subPath,
    namespace: p.namespace,
    ready: p.ready,
    url: p.url,
    isRemote: p.isRemote,
    remoteSync: p.remoteSync
      ? {
          lastSyncTime: p.remoteSync.lastSyncTime,
          lastSyncError: p.remoteSync.lastSyncError,
          remoteTitle: p.remoteSync.remoteTitle,
          fqdnCount: p.remoteSync.fqdnCount,
        }
      : undefined,
  };
}

export async function listPortals(): Promise<Portal[]> {
  const client = createClient(PortalService, createTransport());
  const request = create(ListPortalsRequestSchema, { namespace: "" });
  const response = await client.listPortals(request);
  return response.portals.map(toDomainPortal);
}
