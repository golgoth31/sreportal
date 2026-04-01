import type { ReactNode } from "react";

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { EmojiText } from "@/features/emoji/ui/EmojiText";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { cn } from "@/lib/utils";

const DEFAULT_TRIGGER_CLASS =
  "truncate max-w-xs text-left cursor-pointer hover:text-foreground transition-colors";

const READONLY_TRIGGER_CLASS =
  "truncate max-w-xs text-left block min-w-0 cursor-default outline-none ring-offset-background transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring rounded-sm";

const TOOLTIP_CONTENT_CLASS =
  "max-w-sm whitespace-pre-wrap break-words";

interface TruncatedTooltipShellProps {
  text: string;
  trigger: ReactNode;
}

function TruncatedTooltipShell({ text, trigger }: TruncatedTooltipShellProps) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{trigger}</TooltipTrigger>
        <TooltipContent side="top" className={TOOLTIP_CONTENT_CLASS}>
          <EmojiText text={text} />
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function TruncatedTooltipReadonly({
  text,
  triggerClassName,
}: Pick<TruncatedCopyTooltipTextProps, "text" | "triggerClassName">) {
  return (
    <TruncatedTooltipShell
      text={text}
      trigger={
        <span
          tabIndex={0}
          className={cn(READONLY_TRIGGER_CLASS, triggerClassName)}
          aria-label={text}
        >
          <EmojiText text={text} />
        </span>
      }
    />
  );
}

function TruncatedTooltipWithCopy({
  text,
  copyAriaLabel,
  triggerClassName,
}: Required<Pick<TruncatedCopyTooltipTextProps, "text" | "copyAriaLabel">> &
  Pick<TruncatedCopyTooltipTextProps, "triggerClassName">) {
  const { copy } = useCopyToClipboard(text);

  return (
    <TruncatedTooltipShell
      text={text}
      trigger={
        <button
          type="button"
          onClick={() => void copy()}
          className={cn(DEFAULT_TRIGGER_CLASS, triggerClassName)}
          aria-label={copyAriaLabel}
        >
          <EmojiText text={text} />
        </button>
      }
    />
  );
}

export interface TruncatedCopyTooltipTextProps {
  text: string;
  /** When false, shows the same tooltip but no copy-on-click (read-only trigger). */
  enableCopy?: boolean;
  /** Used when `enableCopy` is true (default). */
  copyAriaLabel?: string;
  /** Classes for the trigger (truncate + max-width must match the column). */
  triggerClassName?: string;
}

export function TruncatedCopyTooltipText({
  text,
  enableCopy = true,
  copyAriaLabel = "Click to copy message",
  triggerClassName,
}: TruncatedCopyTooltipTextProps) {
  if (!text.trim()) {
    return null;
  }

  if (!enableCopy) {
    return (
      <TruncatedTooltipReadonly
        text={text}
        triggerClassName={triggerClassName}
      />
    );
  }

  return (
    <TruncatedTooltipWithCopy
      text={text}
      copyAriaLabel={copyAriaLabel}
      triggerClassName={triggerClassName}
    />
  );
}
