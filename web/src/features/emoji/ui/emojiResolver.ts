import emojis from "emojibase-data/en/data.json";
import shortcodes from "emojibase-data/en/shortcodes/github.json";

// Build a reverse map: shortcode (string) → unicode emoji character.
// This runs once at module load time and is cached.
const shortcodeToUnicode = buildShortcodeMap();

function buildShortcodeMap(): Map<string, string> {
  const hexToEmoji = new Map<string, string>();
  for (const entry of emojis) {
    hexToEmoji.set(entry.hexcode, entry.emoji);
  }

  const result = new Map<string, string>();
  for (const [hexcode, codes] of Object.entries(
    shortcodes as Record<string, string | string[]>,
  )) {
    const unicode = hexToEmoji.get(hexcode);
    if (!unicode) continue;

    const codeList = Array.isArray(codes) ? codes : [codes];
    for (const code of codeList) {
      result.set(code, unicode);
    }
  }

  return result;
}

/** Resolve a standard emoji shortcode (without colons) to its unicode character. */
export function resolveStandardEmoji(shortcode: string): string | undefined {
  return shortcodeToUnicode.get(shortcode);
}
