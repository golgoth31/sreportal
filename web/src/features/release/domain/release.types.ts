export interface ReleaseEntry {
  readonly type: string;
  readonly version?: string;
  readonly origin: string;
  readonly date: string; // ISO 8601
  readonly author: string;
  readonly message: string;
  readonly link: string;
}

export interface ReleasesDay {
  readonly day: string; // YYYY-MM-DD
  readonly entries: readonly ReleaseEntry[];
  readonly previousDay: string;
  readonly nextDay: string;
}

export interface ReleaseTypeConfig {
  readonly name: string;
  readonly color: string; // CSS color value (e.g., "#3b82f6")
}

/** Default type colors used when the server does not provide any. */
export const DEFAULT_TYPE_COLORS: readonly ReleaseTypeConfig[] = [
  { name: "release", color: "#3b82f6" },
  { name: "rollback", color: "#f97316" },
  { name: "hotfix", color: "#ef4444" },
  { name: "canary", color: "#eab308" },
  { name: "feature-flag", color: "#a855f7" },
  { name: "feature flag", color: "#a855f7" },
  { name: "config", color: "#14b8a6" },
  { name: "migration", color: "#6366f1" },
  { name: "infra", color: "#06b6d4" },
] as const;

export interface ReleaseDays {
  readonly days: readonly string[]; // sorted YYYY-MM-DD strings
  readonly ttlDays: number; // TTL window in days
  readonly types: readonly ReleaseTypeConfig[];
}

/** Case-insensitive keyword search across all text fields of an entry. */
export function entryMatchesSearch(
  entry: ReleaseEntry,
  term: string,
): boolean {
  const lower = term.toLowerCase();
  return (
    entry.type.toLowerCase().includes(lower) ||
    (entry.version ?? "").toLowerCase().includes(lower) ||
    entry.origin.toLowerCase().includes(lower) ||
    entry.author.toLowerCase().includes(lower) ||
    entry.message.toLowerCase().includes(lower)
  );
}

/** Sort entries by date descending (most recent first). */
export function sortEntriesByDate(
  entries: readonly ReleaseEntry[],
): readonly ReleaseEntry[] {
  return [...entries].sort((a, b) => b.date.localeCompare(a.date));
}

/** Filter entries by keyword. Returns the original array reference when term is empty. */
export function filterEntries(
  entries: readonly ReleaseEntry[],
  search: string,
): readonly ReleaseEntry[] {
  if (!search) return entries;
  return entries.filter((e) => entryMatchesSearch(e, search));
}

/** Format an ISO date string to a short time (HH:MM) in the given IANA timezone. */
export function formatEntryTime(iso: string, timeZone = "UTC"): string {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone,
  });
}

/** Common IANA timezones for the timezone selector. */
export const COMMON_TIMEZONES: readonly string[] = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Sao_Paulo",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Moscow",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Pacific/Auckland",
] as const;
