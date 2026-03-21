import { create } from "@bufbuild/protobuf";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";

import { RemoteSyncStatusSchema } from "@/gen/sreportal/v1/portal_pb";
import { listPortals } from "./portalApi";
import {
  listPortalsResponseJson,
  samplePortal,
} from "@/test/msw/connectJson";
import { listPortalsPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";

describe("listPortals", () => {
  it("maps remote_sync to domain remoteSync when present", async () => {
    server.use(
      http.post(listPortalsPath, () =>
        HttpResponse.json(
          listPortalsResponseJson([
            samplePortal({
              name: "edge",
              title: "Edge",
              isRemote: true,
              url: "https://remote.example",
              remoteSync: create(RemoteSyncStatusSchema, {
                lastSyncTime: "2024-01-01T00:00:00Z",
                lastSyncError: "",
                remoteTitle: "Remote title",
                fqdnCount: 42,
              }),
            }),
          ]),
        ),
      ),
    );

    const portals = await listPortals();

    expect(portals).toHaveLength(1);
    expect(portals[0]).toMatchObject({
      name: "edge",
      title: "Edge",
      isRemote: true,
      url: "https://remote.example",
      remoteSync: {
        lastSyncTime: "2024-01-01T00:00:00Z",
        lastSyncError: "",
        remoteTitle: "Remote title",
        fqdnCount: 42,
      },
    });
  });
});
