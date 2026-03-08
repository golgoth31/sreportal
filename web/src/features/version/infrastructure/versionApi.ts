import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import {
  GetVersionRequestSchema,
  VersionService,
} from "@/gen/sreportal/v1/version_pb";
import type { VersionInfo } from "../domain/version.types";

const transport = createConnectTransport({ baseUrl: window.location.origin });
const client = createClient(VersionService, transport);

export async function getVersion(): Promise<VersionInfo> {
  const request = create(GetVersionRequestSchema, {});
  const response = await client.getVersion(request);
  return {
    version: response.version,
    commit: response.commit,
    date: response.date,
  };
}
