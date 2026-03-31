import { useMemo, type ReactNode } from "react";

import { useCustomEmojis } from "../hooks/useCustomEmojis";
import { resolveStandardEmoji } from "./emojiResolver";

const SHORTCODE_REGEX = /:([a-zA-Z0-9_+-]+):/g;

interface EmojiTextProps {
  text: string;
  className?: string;
}

export function EmojiText({ text, className }: EmojiTextProps) {
  const { data: customEmojis = {} } = useCustomEmojis();

  const rendered = useMemo(
    () => resolveEmojis(text, customEmojis),
    [text, customEmojis],
  );

  return <span className={className}>{rendered}</span>;
}

function resolveEmojis(
  text: string,
  customEmojis: Record<string, string>,
): ReactNode[] {
  const parts: ReactNode[] = [];
  let lastIndex = 0;

  for (const match of text.matchAll(SHORTCODE_REGEX)) {
    const shortcode = match[1];
    const matchStart = match.index;
    if (shortcode === undefined || matchStart === undefined) continue;
    const matchEnd = matchStart + match[0].length;

    // Push preceding text
    if (matchStart > lastIndex) {
      parts.push(text.slice(lastIndex, matchStart));
    }

    // Try custom emoji (Slack) first
    const customURL = customEmojis[shortcode];
    if (customURL) {
      parts.push(
        <img
          key={matchStart}
          src={customURL}
          alt={`:${shortcode}:`}
          className="inline-block h-5 w-5 align-text-bottom"
          loading="lazy"
        />,
      );
    } else {
      // Try standard emoji (emojibase)
      const unicode = resolveStandardEmoji(shortcode);
      if (unicode) {
        parts.push(
          <span key={matchStart} aria-label={shortcode} role="img">
            {unicode}
          </span>,
        );
      } else {
        // Unknown shortcode — leave as-is
        parts.push(match[0]);
      }
    }

    lastIndex = matchEnd;
  }

  // Push remaining text
  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  return parts.length > 0 ? parts : [text];
}
