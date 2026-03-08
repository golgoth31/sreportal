/**
 * Domain types for Alertmanager alerts.
 * No React or infrastructure dependencies.
 */

export type AlertState = "active" | "suppressed" | "unprocessed";

export interface Alert {
  readonly fingerprint: string;
  readonly labels: Readonly<Record<string, string>>;
  readonly annotations: Readonly<Record<string, string>>;
  readonly state: AlertState;
  readonly startsAt: string;
  readonly endsAt: string | undefined;
  readonly updatedAt: string;
}

export interface AlertmanagerResource {
  readonly name: string;
  readonly namespace: string;
  readonly portalRef: string;
  readonly localUrl: string;
  readonly remoteUrl: string;
  readonly alerts: readonly Alert[];
  readonly lastReconcileTime: string | undefined;
  readonly ready: boolean;
}

export function getAlertName(alert: Alert): string {
  return alert.labels["alertname"] ?? "";
}

export function formatAlertTime(iso: string): string {
  try {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
  } catch {
    return iso;
  }
}
