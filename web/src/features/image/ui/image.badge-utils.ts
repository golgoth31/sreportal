import { formatDistanceToNow } from "date-fns";

import type { ChangeType, TagType } from "../domain/image.types";

export function tagTypeBadgeClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    commit:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    digest:
      "border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/30 dark:text-green-300",
    latest:
      "border-red-200 bg-red-100 text-red-800 dark:border-red-800 dark:bg-red-900/30 dark:text-red-300",
    other:
      "border-gray-200 bg-gray-100 text-gray-700 dark:border-gray-700 dark:bg-gray-800/40 dark:text-gray-300",
  };
  return classes[tagType];
}

export function tagTypeBadgeMutedClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-blue-200/70 bg-transparent text-blue-700/70 hover:bg-blue-50 hover:text-blue-800 dark:border-blue-800/50 dark:text-blue-400/70 dark:hover:bg-blue-900/20 dark:hover:text-blue-300",
    commit:
      "border-blue-200/70 bg-transparent text-blue-700/70 hover:bg-blue-50 hover:text-blue-800 dark:border-blue-800/50 dark:text-blue-400/70 dark:hover:bg-blue-900/20 dark:hover:text-blue-300",
    digest:
      "border-green-200/70 bg-transparent text-green-700/70 hover:bg-green-50 hover:text-green-800 dark:border-green-800/50 dark:text-green-400/70 dark:hover:bg-green-900/20 dark:hover:text-green-300",
    latest:
      "border-red-200/70 bg-transparent text-red-700/70 hover:bg-red-50 hover:text-red-800 dark:border-red-800/50 dark:text-red-400/70 dark:hover:bg-red-900/20 dark:hover:text-red-300",
    other:
      "border-gray-200/70 bg-transparent text-gray-600/70 hover:bg-gray-50 hover:text-gray-800 dark:border-gray-700/50 dark:text-gray-400/70 dark:hover:bg-gray-800/30 dark:hover:text-gray-200",
  };
  return classes[tagType];
}

/** Returns null when changeType is undefined or "unspecified" (no badge). */
export function changeTypeBadgeClass(ct: ChangeType | undefined): string | null {
  if (!ct || ct === "unspecified") return null;
  const classes: Record<Exclude<ChangeType, "unspecified">, string> = {
    none: "border-gray-200 bg-gray-100 text-gray-700 dark:border-gray-700 dark:bg-gray-800/40 dark:text-gray-300",
    mutated:
      "border-orange-200 bg-orange-100 text-orange-800 dark:border-orange-700 dark:bg-orange-900/30 dark:text-orange-300",
    injected:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
  };
  return classes[ct];
}

export function formatRelativeTime(isoString: string | undefined): string | null {
  if (!isoString) return null;
  try {
    return formatDistanceToNow(new Date(isoString), { addSuffix: true });
  } catch {
    return null;
  }
}
