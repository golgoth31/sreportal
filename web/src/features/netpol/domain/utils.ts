/**
 * Shared presentation utilities for network policy views.
 * No React dependencies — pure functions and constants.
 */

/** Palette for namespace/group badges (deterministic by hash). */
export const GROUP_PALETTE = [
  "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  "bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200",
  "bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200",
  "bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200",
  "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  "bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200",
] as const;

/** Deterministic color class for a group/namespace string. */
export function groupColor(group: string): string {
  let hash = 0;
  for (let i = 0; i < group.length; i++) {
    hash = (hash * 31 + group.charCodeAt(i)) | 0;
  }
  return GROUP_PALETTE[Math.abs(hash) % GROUP_PALETTE.length]!;
}

import type { NodeType } from "./netpol.types";

/** Badge colors for node types (service, database, etc.). */
export const NODE_TYPE_COLORS: Record<NodeType, string> = {
  service: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
  database: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
  messaging:
    "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300",
  cron: "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
  external:
    "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
};

/** Maximum number of search results shown in impact analysis. */
export const MAX_SEARCH_RESULTS = 50;

/** Maximum number of top cross-namespace flows displayed. */
export const MAX_TOP_FLOWS = 20;

/** Deduplicate edges by from|to|edgeType key. */
export function dedup<
  T extends { readonly from: string; readonly to: string; readonly edgeType: string },
>(edges: readonly T[]): T[] {
  const seen = new Set<string>();
  return edges.filter((e) => {
    const key = `${e.from}|${e.to}|${e.edgeType}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
