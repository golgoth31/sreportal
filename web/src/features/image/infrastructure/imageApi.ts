import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  ChangeType as ProtoChangeType,
  ImageService,
  ListImagesRequestSchema,
  type Image as ProtoImage,
  type WorkloadRef as ProtoWorkloadRef,
} from "@/gen/sreportal/v1/image_pb";
import type { ChangeType, ContainerSource, Image, WorkloadRef } from "../domain/image.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(ImageService, transport);

function toDomainSource(value: string): ContainerSource {
  return value === "pod" ? "pod" : "spec";
}

function toDomainWorkload(w: ProtoWorkloadRef): WorkloadRef {
  return {
    kind: w.kind,
    namespace: w.namespace,
    name: w.name,
    container: w.container,
    source: toDomainSource(w.source),
  };
}

function toDomainChangeType(ct: ProtoChangeType): ChangeType {
  switch (ct) {
    case ProtoChangeType.NONE:
      return "none";
    case ProtoChangeType.MUTATED:
      return "mutated";
    case ProtoChangeType.INJECTED:
      return "injected";
    default:
      return "unspecified";
  }
}

function toDomainImage(i: ProtoImage): Image {
  return {
    registry: i.registry,
    repository: i.repository,
    tag: i.tag,
    tagType: i.tagType as Image["tagType"],
    workloads: i.workloads.map(toDomainWorkload),
    latestVersion: i.latestVersion || undefined,
    latestCheckedAt: i.latestCheckedAt
      ? new Date(
          Number(i.latestCheckedAt.seconds) * 1000 +
            Math.round(i.latestCheckedAt.nanos / 1_000_000),
        ).toISOString()
      : undefined,
    latestError: i.latestError || undefined,
    upgradeAvailable: i.upgradeAvailable || undefined,
    changeType: toDomainChangeType(i.changeType),
    originalImage: i.originalImage || undefined,
  };
}

export async function listImages(portal: string): Promise<Image[]> {
  const req = create(ListImagesRequestSchema, {
    portal,
    search: "",
    registryFilter: "",
    tagTypeFilter: "",
  });
  const res = await client.listImages(req);
  return res.images.map(toDomainImage);
}
