import type { DeployState } from "../domain/deploystatus.types";

/** Tailwind classes for the active state badge. */
export function stateBadgeClass(state: DeployState): string {
  switch (state) {
    case "ok":
      return "border-emerald-300 bg-emerald-100 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
    case "behind":
      return "border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
    case "unresolved":
      return "border-gray-300 bg-gray-100 text-gray-700 dark:border-gray-600 dark:bg-gray-800/50 dark:text-gray-300";
    case "error":
      return "border-red-300 bg-red-100 text-red-800 dark:border-red-700 dark:bg-red-900/30 dark:text-red-300";
  }
}

/** Human-readable label for a deploy state. */
export function stateLabel(state: DeployState): string {
  switch (state) {
    case "ok":
      return "up to date";
    case "behind":
      return "behind";
    case "unresolved":
      return "unresolved";
    case "error":
      return "error";
  }
}
