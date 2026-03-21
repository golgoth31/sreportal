import { create, toJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

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
import {
  ListReleasesResponseSchema,
  ReleaseEntrySchema,
  type ReleaseEntry,
} from "@/gen/sreportal/v1/release_pb";

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

export function sampleReleaseEntry(
  overrides: Partial<ReleaseEntry> & Pick<ReleaseEntry, "type" | "version" | "origin">,
): ReleaseEntry {
  return create(ReleaseEntrySchema, {
    date: timestampFromDate(new Date("2026-03-21T10:00:00Z")),
    author: "",
    message: "",
    link: "",
    ...overrides,
  });
}

export function listReleasesResponseJson(
  day: string,
  entries: ReleaseEntry[],
  previousDay = "",
  nextDay = "",
) {
  const message = create(ListReleasesResponseSchema, {
    day,
    entries,
    previousDay,
    nextDay,
    nextPageToken: "",
  });
  return toJson(ListReleasesResponseSchema, message);
}
