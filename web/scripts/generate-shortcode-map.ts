/**
 * Build-time script: generates a pre-built shortcode→emoji map so the full
 * emojibase-data JSON (~750 KB) never ships to the client bundle.
 *
 * Output: src/features/emoji/ui/shortcode-map.json (~36 KB)
 *
 * Usage: npx tsx scripts/generate-shortcode-map.ts
 */
import { writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

import compactData from "emojibase-data/en/compact.json" with { type: "json" };
import githubShortcodes from "emojibase-data/en/shortcodes/github.json" with { type: "json" };

const __dirname = dirname(fileURLToPath(import.meta.url));
const OUTPUT = resolve(
  __dirname,
  "../src/features/emoji/ui/shortcode-map.json",
);

// Map hexcode → unicode character
const hexToEmoji = new Map<string, string>();
for (const entry of compactData) {
  hexToEmoji.set(entry.hexcode, entry.unicode);
}

// Build shortcode → unicode map
const shortcodeMap: Record<string, string> = {};
for (const [hexcode, codes] of Object.entries(
  githubShortcodes as Record<string, string | string[]>,
)) {
  const unicode = hexToEmoji.get(hexcode);
  if (!unicode) continue;

  const codeList = Array.isArray(codes) ? codes : [codes];
  for (const code of codeList) {
    shortcodeMap[code] = unicode;
  }
}

writeFileSync(OUTPUT, JSON.stringify(shortcodeMap), "utf-8");
console.log(
  `Generated shortcode map: ${Object.keys(shortcodeMap).length} entries → ${OUTPUT}`,
);
