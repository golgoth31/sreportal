import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  EmojiService,
  ListCustomEmojisRequestSchema,
} from "@/gen/sreportal/v1/emoji_pb";
import type { CustomEmojiMap } from "../domain/emoji.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(EmojiService, transport);

export async function listCustomEmojis(): Promise<CustomEmojiMap> {
  const request = create(ListCustomEmojisRequestSchema, {});
  const response = await client.listCustomEmojis(request);
  return { ...response.emojis };
}
