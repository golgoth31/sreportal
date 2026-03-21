export interface ReleaseEntry {
  readonly type: string;
  readonly version: string;
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

/** Case-insensitive keyword search across all text fields of an entry. */
export function entryMatchesSearch(
  entry: ReleaseEntry,
  term: string,
): boolean {
  const lower = term.toLowerCase();
  return (
    entry.type.toLowerCase().includes(lower) ||
    entry.version.toLowerCase().includes(lower) ||
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

/** Format an ISO date string to a short locale time (HH:MM). */
export function formatEntryTime(iso: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}
