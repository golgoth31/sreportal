import { create } from "@bufbuild/protobuf";
import { ConnectError, Code, createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  ListReleaseDaysRequestSchema,
  ListReleasesRequestSchema,
  ReleaseService,
  type ReleaseEntry as ProtoEntry,
} from "@/gen/sreportal/v1/release_pb";
import type {
  ReleaseEntry,
  ReleaseDays,
  ReleasesDay,
} from "../domain/release.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(ReleaseService, transport);

function timestampToIso(
  ts: { seconds?: bigint; nanos?: number } | undefined,
): string {
  if (ts == null || ts.seconds == null) return "";
  const ms = Number(ts.seconds) * 1000 + (ts.nanos ?? 0) / 1e6;
  return new Date(ms).toISOString();
}

function toDomainEntry(e: ProtoEntry): ReleaseEntry {
  return {
    type: e.type,
    version: e.version || undefined,
    origin: e.origin,
    date: e.date ? timestampToIso(e.date) : "",
    author: e.author,
    message: e.message,
    link: e.link,
  };
}

export async function listReleaseDays(portal: string): Promise<ReleaseDays> {
  const request = create(ListReleaseDaysRequestSchema, { portal });
  const response = await client.listReleaseDays(request);
  return {
    days: [...response.days],
    ttlDays: response.ttlDays,
    types: response.types.map((t) => ({ name: t.name, color: t.color })),
  };
}

export async function listReleases(
  day = "",
  portal: string,
): Promise<ReleasesDay> {
  try {
    const request = create(ListReleasesRequestSchema, { day, portal });
    const response = await client.listReleases(request);
    return {
      day: response.day,
      entries: response.entries.map(toDomainEntry),
      previousDay: response.previousDay,
      nextDay: response.nextDay,
    };
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.NotFound) {
      return { day, entries: [], previousDay: "", nextDay: "" };
    }
    throw err;
  }
}
