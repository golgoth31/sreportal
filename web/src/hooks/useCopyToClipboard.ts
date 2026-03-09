import { useCallback, useState } from "react";
import { toast } from "sonner";

export interface UseCopyToClipboardReturn {
  copied: boolean;
  copy: () => Promise<void>;
}

export function useCopyToClipboard(text: string): UseCopyToClipboardReturn {
  const [copied, setCopied] = useState(false);

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  }, [text]);

  return { copied, copy };
}
