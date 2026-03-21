import { http, HttpResponse } from "msw";

import {
  listFqdnsResponseJson,
  listPortalsResponseJson,
  listReleasesResponseJson,
  sampleFqdn,
  samplePortal,
} from "./connectJson";

/** Connect method URL suffix (see createMethodUrl in @connectrpc/connect). */
export const listFqdnsPath = /\/sreportal\.v1\.DNSService\/ListFQDNs$/;
export const listPortalsPath = /\/sreportal\.v1\.PortalService\/ListPortals$/;
export const listReleasesPath = /\/sreportal\.v1\.ReleaseService\/ListReleases$/;

export const defaultHandlers = [
  http.post(listFqdnsPath, () =>
    HttpResponse.json(
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
    HttpResponse.json(
      listPortalsResponseJson([
        samplePortal({ name: "main", title: "Main", main: true }),
      ]),
    ),
  ),
  http.post(listReleasesPath, () =>
    HttpResponse.json(
      listReleasesResponseJson("", []),
    ),
  ),
];
