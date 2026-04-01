import type { ComponentStatus, PlatformComponent } from "./types";

const statusSeverityOrder: Record<ComponentStatus, number> = {
  major_outage: 6,
  partial_outage: 5,
  degraded: 4,
  maintenance: 3,
  unknown: 2,
  operational: 1,
};

export function computeGlobalStatus(
  components: PlatformComponent[]
): ComponentStatus {
  if (components.length === 0) return "unknown";

  let worst: ComponentStatus = "operational";

  for (const comp of components) {
    const status = comp.computedStatus || comp.declaredStatus;
    if (statusSeverityOrder[status] > statusSeverityOrder[worst]) {
      worst = status;
    }
  }

  return worst;
}

export function getStatusColor(status: ComponentStatus): string {
  switch (status) {
    case "operational":
      return "text-green-600 bg-green-50 border-green-200 dark:text-green-400 dark:bg-green-950 dark:border-green-800";
    case "degraded":
      return "text-yellow-600 bg-yellow-50 border-yellow-200 dark:text-yellow-400 dark:bg-yellow-950 dark:border-yellow-800";
    case "partial_outage":
      return "text-orange-600 bg-orange-50 border-orange-200 dark:text-orange-400 dark:bg-orange-950 dark:border-orange-800";
    case "major_outage":
      return "text-red-600 bg-red-50 border-red-200 dark:text-red-400 dark:bg-red-950 dark:border-red-800";
    case "maintenance":
      return "text-blue-600 bg-blue-50 border-blue-200 dark:text-blue-400 dark:bg-blue-950 dark:border-blue-800";
    default:
      return "text-gray-600 bg-gray-50 border-gray-200 dark:text-gray-400 dark:bg-gray-900 dark:border-gray-700";
  }
}

export function getStatusLabel(status: ComponentStatus): string {
  switch (status) {
    case "operational":
      return "Operational";
    case "degraded":
      return "Degraded";
    case "partial_outage":
      return "Partial Outage";
    case "major_outage":
      return "Major Outage";
    case "maintenance":
      return "Maintenance";
    default:
      return "Unknown";
  }
}

export function getStatusDotColor(status: ComponentStatus): string {
  switch (status) {
    case "operational":
      return "bg-green-500";
    case "degraded":
      return "bg-yellow-500";
    case "partial_outage":
      return "bg-orange-500";
    case "major_outage":
      return "bg-red-500";
    case "maintenance":
      return "bg-blue-500";
    default:
      return "bg-gray-400";
  }
}

export function getGlobalStatusMessage(status: ComponentStatus): string {
  switch (status) {
    case "operational":
      return "All Systems Operational";
    case "degraded":
      return "Some Systems Degraded";
    case "partial_outage":
      return "Partial System Outage";
    case "major_outage":
      return "Major Outage";
    case "maintenance":
      return "Maintenance In Progress";
    default:
      return "System Status Unknown";
  }
}
