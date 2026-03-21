export type MetricType = "COUNTER" | "GAUGE" | "HISTOGRAM";

export interface HistogramBucket {
  readonly cumulativeCount: number;
  readonly upperBound: number;
}

export interface HistogramValue {
  readonly sampleCount: number;
  readonly sampleSum: number;
  readonly buckets: readonly HistogramBucket[];
}

export interface Metric {
  readonly labels: Readonly<Record<string, string>>;
  readonly value: number;
  readonly histogram?: HistogramValue;
}

export interface MetricFamily {
  readonly name: string;
  readonly help: string;
  readonly type: MetricType;
  readonly metrics: readonly Metric[];
}

const NAMESPACE_PREFIX = "sreportal_";

/** Extracts the subsystem from a sreportal metric name (e.g. "sreportal_dns_fqdns_total" → "dns"). */
export function extractSubsystem(name: string): string {
  if (!name.startsWith(NAMESPACE_PREFIX)) return "";
  const rest = name.slice(NAMESPACE_PREFIX.length);
  const idx = rest.indexOf("_");
  return idx === -1 ? rest : rest.slice(0, idx);
}

/** Strips the "sreportal_" prefix from a metric name. */
export function formatMetricName(name: string): string {
  return name.startsWith(NAMESPACE_PREFIX)
    ? name.slice(NAMESPACE_PREFIX.length)
    : name;
}

/** Filters metric families by search substring and/or subsystem. */
export function filterFamilies(
  families: readonly MetricFamily[],
  search: string,
  subsystem: string,
): MetricFamily[] {
  const searchLower = search.toLowerCase();
  return families.filter((f) => {
    if (searchLower && !f.name.toLowerCase().includes(searchLower)) {
      return false;
    }
    if (subsystem && extractSubsystem(f.name) !== subsystem) {
      return false;
    }
    return true;
  });
}

/** Extracts sorted unique subsystems from a list of metric families. */
export function extractSubsystems(
  families: readonly MetricFamily[],
): string[] {
  const set = new Set<string>();
  for (const f of families) {
    const sub = extractSubsystem(f.name);
    if (sub) set.add(sub);
  }
  return [...set].sort();
}
