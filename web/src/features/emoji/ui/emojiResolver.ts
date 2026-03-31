// Pre-built at build time by scripts/generate-shortcode-map.ts (~36 KB vs ~750 KB raw emojibase-data).
import shortcodeMap from "./shortcode-map.json";

/** Resolve a standard emoji shortcode (without colons) to its unicode character. */
export function resolveStandardEmoji(shortcode: string): string | undefined {
  return (shortcodeMap as Record<string, string>)[shortcode];
}
