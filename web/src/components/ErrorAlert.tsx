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
      className="flex items-center gap-3 rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 text-destructive"
    >
      <AlertCircleIcon className="size-5 shrink-0" />
      <div>
        <p className="font-medium text-sm">{title}</p>
        <p className="text-xs mt-0.5 opacity-80">{message}</p>
      </div>
    </div>
  );
}
