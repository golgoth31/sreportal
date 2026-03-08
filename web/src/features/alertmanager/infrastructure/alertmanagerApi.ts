import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import {
  AlertmanagerService,
  ListAlertsRequestSchema,
  type Alert as ProtoAlert,
  type AlertmanagerResource as ProtoResource,
} from "@/gen/sreportal/v1/alertmanager_pb";
import type { Alert, AlertmanagerResource } from "../domain/alertmanager.types";

const transport = createConnectTransport({ baseUrl: window.location.origin });

function timestampToIso(
  ts: { seconds?: bigint; nanos?: number } | undefined
): string {
  if (ts == null || ts.seconds == null) return "";
  const ms = Number(ts.seconds) * 1000 + (ts.nanos ?? 0) / 1e6;
  return new Date(ms).toISOString();
}

function toDomainAlert(a: ProtoAlert): Alert {
  return {
    fingerprint: a.fingerprint,
    labels: { ...a.labels },
    annotations: { ...(a.annotations ?? {}) },
    state: a.state,
    startsAt: timestampToIso(a.startsAt),
    endsAt: a.endsAt ? timestampToIso(a.endsAt) : undefined,
    updatedAt: timestampToIso(a.updatedAt),
  };
}

function toDomainResource(r: ProtoResource): AlertmanagerResource {
  return {
    name: r.name,
    namespace: r.namespace,
    portalRef: r.portalRef,
    localUrl: r.localUrl,
    remoteUrl: r.remoteUrl,
    alerts: r.alerts.map(toDomainAlert),
    lastReconcileTime: r.lastReconcileTime ? timestampToIso(r.lastReconcileTime) : undefined,
    ready: r.ready,
  };
}

export interface ListAlertsParams {
  portal?: string;
  namespace?: string;
  search?: string;
  state?: string;
}

export async function listAlerts(params: ListAlertsParams = {}): Promise<AlertmanagerResource[]> {
  const client = createClient(AlertmanagerService, transport);
  const request = create(ListAlertsRequestSchema, {
    portal: params.portal ?? "",
    namespace: params.namespace ?? "",
    search: params.search ?? "",
    state: params.state ?? "",
  });
  const response = await client.listAlerts(request);
  return response.alertmanagers.map(toDomainResource);
}
