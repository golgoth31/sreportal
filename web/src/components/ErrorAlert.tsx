import { AlertCircleIcon } from "lucide-react";

interface ErrorAlertProps {
  title: string;
  error: unknown;
}

export function ErrorAlert({ title, error }: ErrorAlertProps) {
  const message = error instanceof Error ? error.message : String(error);

  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 px-4 py-3 text-destructive"
    >
      <AlertCircleIcon className="size-5 shrink-0 mt-0.5" />
      <div className="min-w-0">
        <p className="font-medium text-sm">{title}</p>
        <p className="text-xs mt-0.5 opacity-80 font-mono break-words">{message}</p>
      </div>
    </div>
  );
}
