import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import {
  DNSService,
  type FQDN,
  ListFQDNsRequestSchema,
  type OriginResourceRef,
} from "@/gen/sreportal/v1/dns_pb";
import type { Fqdn, OriginRef } from "../domain/dns.types";

function createTransport() {
  return createConnectTransport({ baseUrl: window.location.origin });
}

function toDomainOriginRef(ref: OriginResourceRef): OriginRef {
  return { kind: ref.kind, namespace: ref.namespace, name: ref.name };
}

function toDomainFqdn(f: FQDN): Fqdn {
  return {
    name: f.name,
    source: f.source,
    groups: [...f.groups],
    description: f.description,
    recordType: f.recordType,
    targets: [...f.targets],
    dnsResourceName: f.dnsResourceName,
    dnsResourceNamespace: f.dnsResourceNamespace,
    originRef: f.originRef ? toDomainOriginRef(f.originRef) : undefined,
    syncStatus: f.syncStatus,
  };
}

export async function listFqdns(portal: string): Promise<Fqdn[]> {
  const client = createClient(DNSService, createTransport());
  const request = create(ListFQDNsRequestSchema, {
    portal,
    namespace: "",
    source: "",
    search: "",
  });
  const response = await client.listFQDNs(request);
  return response.fqdns.map(toDomainFqdn);
}
