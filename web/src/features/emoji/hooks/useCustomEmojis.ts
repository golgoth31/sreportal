import { useQuery } from "@tanstack/react-query";

import { listCustomEmojis } from "../infrastructure/emojiApi";
import type { CustomEmojiMap } from "../domain/emoji.types";

export function useCustomEmojis() {
  return useQuery<CustomEmojiMap>({
    queryKey: ["customEmojis"],
    queryFn: listCustomEmojis,
    staleTime: 5 * 60_000, // 5 minutes — backend refreshes every 24h
    placeholderData: {}, // empty map while loading, no flicker
  });
}
