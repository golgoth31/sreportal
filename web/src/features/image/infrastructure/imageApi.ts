import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  ImageService,
  ListImagesRequestSchema,
  type WorkloadRef as ProtoWorkloadRef,
  type Image as ProtoImage,
} from "@/gen/sreportal/v1/image_pb";
import type { ContainerSource, Image, WorkloadRef } from "../domain/image.types";

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

function toDomainImage(i: ProtoImage): Image {
  return {
    registry: i.registry,
    repository: i.repository,
    tag: i.tag,
    tagType: i.tagType as Image["tagType"],
    workloads: i.workloads.map(toDomainWorkload),
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
