import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

export interface UseCopyToClipboardReturn {
  copied: boolean;
  copy: () => Promise<void>;
}

export function useCopyToClipboard(text: string): UseCopyToClipboardReturn {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timerRef.current), []);

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      toast.success("Copied to clipboard");
      clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  }, [text]);

  return { copied, copy };
}
