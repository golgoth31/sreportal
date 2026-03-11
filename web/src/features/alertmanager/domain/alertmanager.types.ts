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
  /** Receivers this alert is routed to */
  readonly receivers: readonly string[];
  /** IDs of silences that suppress this alert */
  readonly silencedBy: readonly string[];
}

/** Returns true when the alert is suppressed by one or more silences */
export function isSilenced(alert: Alert): boolean {
  return alert.silencedBy.length > 0;
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
  readonly silences?: readonly Silence[];
}

export interface Silence {
  readonly id: string;
  readonly matchers: readonly Matcher[];
  readonly startsAt: string;
  readonly endsAt: string;
  readonly status: string;
  readonly createdBy: string;
  readonly comment: string;
  readonly updatedAt: string;
}

export interface Matcher {
  readonly name: string;
  readonly value: string;
  readonly isRegex: boolean;
}

export interface AlertGroup {
  readonly name: string;
  readonly alerts: readonly Alert[];
}

export function getAlertName(alert: Alert): string {
  return alert.labels["alertname"] ?? "";
}

export function groupAlertsByName(alerts: readonly Alert[]): AlertGroup[] {
  const grouped = new Map<string, Alert[]>();
  for (const alert of alerts) {
    const name = getAlertName(alert) || alert.fingerprint.slice(0, 8);
    const existing = grouped.get(name);
    if (existing) {
      existing.push(alert);
    } else {
      grouped.set(name, [alert]);
    }
  }
  return Array.from(grouped.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, groupAlerts]) => ({ name, alerts: groupAlerts }));
}

export function formatAlertTime(iso: string): string {
  try {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
  } catch {
    return iso;
  }
}
