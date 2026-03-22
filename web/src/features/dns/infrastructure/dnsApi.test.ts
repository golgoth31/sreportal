import { http } from "msw";
import { describe, expect, it } from "vitest";

import { listFqdns } from "./dnsApi";
import {
  listFqdnsResponseJson,
  sampleFqdn,
} from "@/test/msw/connectJson";
import { grpcWebResponse, listFqdnsPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";

describe("listFqdns", () => {
  it("maps gRPC-Web response to domain Fqdn objects", async () => {
    server.use(
      http.post(listFqdnsPath, () =>
        grpcWebResponse(
          listFqdnsResponseJson([
            sampleFqdn({
              name: "svc.cluster.local",
              source: "manual",
              groups: ["internal"],
              description: "In-cluster",
              recordType: "CNAME",
              targets: ["target.example.com"],
              dnsResourceName: "dns-1",
              dnsResourceNamespace: "kube-system",
              syncStatus: "sync",
            }),
          ]),
        ),
      ),
    );

    const rows = await listFqdns("main");

    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      name: "svc.cluster.local",
      source: "manual",
      groups: ["internal"],
      description: "In-cluster",
      recordType: "CNAME",
      targets: ["target.example.com"],
      dnsResourceName: "dns-1",
      dnsResourceNamespace: "kube-system",
      syncStatus: "sync",
    });
  });

  it("sends portal name in the ListFQDNs request", async () => {
    let receivedRequest = false;
    server.use(
      http.post(listFqdnsPath, () => {
        receivedRequest = true;
        return grpcWebResponse(listFqdnsResponseJson([]));
      }),
    );

    await listFqdns("staging");

    expect(receivedRequest).toBe(true);
  });
});
