import { describe, expect, it, vi } from "vitest";

import {
  formatAlertTime,
  getAlertName,
  groupAlertsByName,
  isSilenced,
  type Alert,
} from "./alertmanager.types";

function mkAlert(overrides: Partial<Alert> & Pick<Alert, "fingerprint">): Alert {
  return {
    fingerprint: overrides.fingerprint,
    labels: overrides.labels ?? {},
    annotations: overrides.annotations ?? {},
    state: overrides.state ?? "active",
    startsAt: overrides.startsAt ?? "2020-01-01T00:00:00.000Z",
    endsAt: overrides.endsAt,
    updatedAt: overrides.updatedAt ?? "2020-01-01T00:00:00.000Z",
    receivers: overrides.receivers ?? [],
    silencedBy: overrides.silencedBy ?? [],
  };
}

describe("isSilenced", () => {
  it("returns true when silencedBy is non-empty", () => {
    expect(
      isSilenced(
        mkAlert({ fingerprint: "fp", silencedBy: ["sil-1"] }),
      ),
    ).toBe(true);
    expect(isSilenced(mkAlert({ fingerprint: "fp", silencedBy: [] }))).toBe(
      false,
    );
  });
});

describe("getAlertName", () => {
  it("returns alertname label when present", () => {
    const a = mkAlert({
      fingerprint: "fp",
      labels: { alertname: "HighCPU" },
    });
    expect(getAlertName(a)).toBe("HighCPU");
  });

  it("returns empty string when alertname is missing", () => {
    const a = mkAlert({ fingerprint: "fp", labels: {} });
    expect(getAlertName(a)).toBe("");
  });
});

describe("groupAlertsByName", () => {
  it("groups by alertname and sorts groups by name", () => {
    const a = mkAlert({
      fingerprint: "fp-a",
      labels: { alertname: "BRule" },
    });
    const b = mkAlert({
      fingerprint: "fp-b",
      labels: { alertname: "ARule" },
    });
    const groups = groupAlertsByName([a, b]);
    expect(groups.map((g) => g.name)).toEqual(["ARule", "BRule"]);
  });

  it("uses fingerprint prefix when alertname is empty", () => {
    const longFp = "abcdefgh12345678";
    const a = mkAlert({ fingerprint: longFp, labels: {} });
    const groups = groupAlertsByName([a]);
    expect(groups[0]?.name).toBe("abcdefgh");
  });
});

describe("formatAlertTime", () => {
  it("returns locale string for valid ISO dates", () => {
    const spy = vi.spyOn(Date.prototype, "toLocaleString").mockReturnValue("ok");
    expect(formatAlertTime("2020-06-15T12:00:00.000Z")).toBe("ok");
    spy.mockRestore();
  });

  it("returns original string when date is invalid", () => {
    expect(formatAlertTime("not-a-date")).toBe("not-a-date");
  });
});
