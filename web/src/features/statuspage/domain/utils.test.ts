import { describe, it, expect } from "vitest";

import type { ComponentStatus, PlatformComponent } from "./types";
import {
  computeGlobalStatus,
  getStatusColor,
  getStatusLabel,
  getGlobalStatusMessage,
} from "./utils";

function component(
  overrides: Partial<PlatformComponent> & { computedStatus: ComponentStatus }
): PlatformComponent {
  return {
    name: "comp-1",
    displayName: "Component 1",
    description: "",
    group: "default",
    link: "",
    portalRef: "main",
    declaredStatus: "operational",
    activeIncidents: 0,
    lastStatusChange: "",
    dailyWorstStatus: [],
    ...overrides,
  };
}

describe("computeGlobalStatus", () => {
  it("returns unknown when no components", () => {
    expect(computeGlobalStatus([])).toBe("unknown");
  });

  it("returns operational when all components are operational", () => {
    const result = computeGlobalStatus([
      component({ computedStatus: "operational" }),
      component({ computedStatus: "operational" }),
    ]);
    expect(result).toBe("operational");
  });

  it("returns the worst status across components", () => {
    const result = computeGlobalStatus([
      component({ computedStatus: "operational" }),
      component({ computedStatus: "degraded" }),
      component({ computedStatus: "partial_outage" }),
    ]);
    expect(result).toBe("partial_outage");
  });

  it("major_outage is the worst status", () => {
    const result = computeGlobalStatus([
      component({ computedStatus: "operational" }),
      component({ computedStatus: "major_outage" }),
    ]);
    expect(result).toBe("major_outage");
  });

  it("maintenance ranks below degraded", () => {
    const result = computeGlobalStatus([
      component({ computedStatus: "maintenance" }),
      component({ computedStatus: "degraded" }),
    ]);
    expect(result).toBe("degraded");
  });

  it("uses computedStatus over declaredStatus", () => {
    const result = computeGlobalStatus([
      component({ declaredStatus: "major_outage", computedStatus: "operational" }),
    ]);
    expect(result).toBe("operational");
  });
});

describe("getStatusColor", () => {
  const allStatuses: ComponentStatus[] = [
    "operational",
    "degraded",
    "partial_outage",
    "major_outage",
    "maintenance",
    "unknown",
  ];

  it.each(allStatuses)("returns a non-empty string for %s", (status) => {
    const result = getStatusColor(status);
    expect(result).toBeTruthy();
    expect(typeof result).toBe("string");
  });

  it("returns green classes for operational", () => {
    expect(getStatusColor("operational")).toContain("green");
  });

  it("returns red classes for major_outage", () => {
    expect(getStatusColor("major_outage")).toContain("red");
  });
});

describe("getStatusLabel", () => {
  it("returns human-readable labels", () => {
    expect(getStatusLabel("operational")).toBe("Operational");
    expect(getStatusLabel("degraded")).toBe("Degraded");
    expect(getStatusLabel("partial_outage")).toBe("Partial Outage");
    expect(getStatusLabel("major_outage")).toBe("Major Outage");
    expect(getStatusLabel("maintenance")).toBe("Maintenance");
    expect(getStatusLabel("unknown")).toBe("Unknown");
  });
});

describe("getGlobalStatusMessage", () => {
  it("returns descriptive messages", () => {
    expect(getGlobalStatusMessage("operational")).toBe("All Systems Operational");
    expect(getGlobalStatusMessage("major_outage")).toBe("Major Outage");
    expect(getGlobalStatusMessage("unknown")).toBe("System Status Unknown");
  });
});
