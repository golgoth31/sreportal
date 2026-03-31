import { create, toBinary, type DescMessage, type MessageShape } from "@bufbuild/protobuf";
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
  ListCustomEmojisResponseSchema,
} from "@/gen/sreportal/v1/emoji_pb";
import {
  ListReleaseDaysResponseSchema,
  ListReleasesResponseSchema,
  ReleaseEntrySchema,
  type ReleaseEntry,
} from "@/gen/sreportal/v1/release_pb";

// ---------------------------------------------------------------------------
// gRPC-Web binary framing helpers
// ---------------------------------------------------------------------------

/**
 * Encode a protobuf message into a gRPC-Web response body.
 *
 * gRPC-Web format:
 *   data frame    = 0x00 + 4-byte big-endian length + serialised protobuf
 *   trailers frame = 0x80 + 4-byte big-endian length + "grpc-status:0\r\n"
 */
function grpcWebFrame<T extends DescMessage>(
  schema: T,
  message: MessageShape<T>,
): Uint8Array {
  const data = toBinary(schema, message);
  const trailers = new TextEncoder().encode("grpc-status:0\r\n");

  const buf = new Uint8Array(1 + 4 + data.length + 1 + 4 + trailers.length);
  const view = new DataView(buf.buffer);
  let offset = 0;

  // Data frame
  buf[offset++] = 0x00;
  view.setUint32(offset, data.length);
  offset += 4;
  buf.set(data, offset);
  offset += data.length;

  // Trailers frame
  buf[offset++] = 0x80;
  view.setUint32(offset, trailers.length);
  offset += 4;
  buf.set(trailers, offset);

  return buf;
}

// ---------------------------------------------------------------------------
// Response builders — return gRPC-Web binary bodies
// ---------------------------------------------------------------------------

export function listFqdnsResponseJson(fqdns: FQDN[]) {
  const message = create(ListFQDNsResponseSchema, {
    fqdns,
    nextPageToken: "",
    totalSize: fqdns.length,
  });
  return grpcWebFrame(ListFQDNsResponseSchema, message);
}

export function listPortalsResponseJson(portals: Portal[]) {
  const message = create(ListPortalsResponseSchema, { portals });
  return grpcWebFrame(ListPortalsResponseSchema, message);
}

export function listReleaseDaysResponseJson(
  days: string[],
  ttlDays = 30,
) {
  const message = create(ListReleaseDaysResponseSchema, { days, ttlDays });
  return grpcWebFrame(ListReleaseDaysResponseSchema, message);
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
  return grpcWebFrame(ListReleasesResponseSchema, message);
}

export function listCustomEmojisResponseJson(
  emojis: Record<string, string>,
) {
  const message = create(ListCustomEmojisResponseSchema, { emojis });
  return grpcWebFrame(ListCustomEmojisResponseSchema, message);
}

// ---------------------------------------------------------------------------
// Sample factory helpers (unchanged)
// ---------------------------------------------------------------------------

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

/** Proto field overrides only — avoid `Partial<ReleaseEntry>` (pulls in `$typeName` and breaks `create()`). */
export type SampleReleaseEntryOverrides = Pick<ReleaseEntry, "type" | "version" | "origin"> &
  Partial<Pick<ReleaseEntry, "date" | "author" | "message" | "link" | "portal">>;

export function sampleReleaseEntry(overrides: SampleReleaseEntryOverrides): ReleaseEntry {
  return create(ReleaseEntrySchema, {
    date: timestampFromDate(new Date("2026-03-21T10:00:00Z")),
    author: "",
    message: "",
    link: "",
    portal: "",
    ...overrides,
  });
}
