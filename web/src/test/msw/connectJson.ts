import { create, toJson } from "@bufbuild/protobuf";

import {
  FQDNSchema,
  ListFQDNsResponseSchema,
  type FQDN,
} from "@/gen/sreportal/v1/dns_pb";
import {
  ListPortalsResponseSchema,
  PortalSchema,
  type Portal,
} from "@/gen/sreportal/v1/portal_pb";

/** JSON body for Connect unary JSON responses (matches @connectrpc/connect-web defaults). */
export function listFqdnsResponseJson(fqdns: FQDN[]) {
  const message = create(ListFQDNsResponseSchema, {
    fqdns,
    nextPageToken: "",
    totalSize: fqdns.length,
  });
  return toJson(ListFQDNsResponseSchema, message);
}

export function sampleFqdn(
  overrides: Partial<FQDN> & Pick<FQDN, "name">,
): FQDN {
  return create(FQDNSchema, {
    source: "external-dns",
    groups: [],
    description: "",
    recordType: "A",
    targets: [],
    dnsResourceName: "dns-sample",
    dnsResourceNamespace: "default",
    syncStatus: "",
    ...overrides,
  });
}

export function listPortalsResponseJson(portals: Portal[]) {
  const message = create(ListPortalsResponseSchema, { portals });
  return toJson(ListPortalsResponseSchema, message);
}

export function samplePortal(
  overrides: Partial<Portal> & Pick<Portal, "name" | "title">,
): Portal {
  return create(PortalSchema, {
    main: false,
    subPath: "",
    namespace: "default",
    ready: true,
    url: "",
    isRemote: false,
    ...overrides,
  });
}
