import { http, HttpResponse } from "msw";

import {
  listCustomEmojisResponseJson,
  listFqdnsResponseJson,
  listPortalsResponseJson,
  listReleaseDaysResponseJson,
  listReleasesResponseJson,
  sampleFqdn,
  samplePortal,
} from "./connectJson";

/** Connect method URL suffix (see createMethodUrl in @connectrpc/connect). */
export const listFqdnsPath = /\/sreportal\.v1\.DNSService\/ListFQDNs$/;
export const listPortalsPath = /\/sreportal\.v1\.PortalService\/ListPortals$/;
export const listReleasesPath = /\/sreportal\.v1\.ReleaseService\/ListReleases$/;
export const listReleaseDaysPath = /\/sreportal\.v1\.ReleaseService\/ListReleaseDays$/;
export const listCustomEmojisPath = /\/sreportal\.v1\.EmojiService\/ListCustomEmojis$/;

/** gRPC-Web Content-Type header. */
const GRPC_WEB_HEADERS = {
  "Content-Type": "application/grpc-web+proto",
};

/** Wrap a gRPC-Web binary body into an MSW HttpResponse. */
export function grpcWebResponse(body: Uint8Array) {
  return new HttpResponse(body, { headers: GRPC_WEB_HEADERS });
}

export const defaultHandlers = [
  http.post(listFqdnsPath, () =>
    grpcWebResponse(
      listFqdnsResponseJson([
        sampleFqdn({
          name: "api.example.com",
          groups: ["default"],
          description: "API",
          syncStatus: "sync",
        }),
      ]),
    ),
  ),
  http.post(listPortalsPath, () =>
    grpcWebResponse(
      listPortalsResponseJson([
        samplePortal({ name: "main", title: "Main", main: true }),
      ]),
    ),
  ),
  http.post(listReleasesPath, () =>
    grpcWebResponse(
      listReleasesResponseJson("", []),
    ),
  ),
  http.post(listReleaseDaysPath, () =>
    grpcWebResponse(
      listReleaseDaysResponseJson([], 30),
    ),
  ),
  http.post(listCustomEmojisPath, () =>
    grpcWebResponse(
      listCustomEmojisResponseJson({}),
    ),
  ),
];
