import { describe, expect, it } from "vitest";

import {
  hasRemoteSyncError,
  portalRefForRoute,
  type Portal,
} from "./portal.types";

function portal(p: Partial<Portal> & Pick<Portal, "name" | "title">): Portal {
  return {
    main: false,
    subPath: "",
    namespace: "default",
    ready: true,
    url: "",
    isRemote: false,
    remoteSync: undefined,
    features: {
      dns: true,
      releases: true,
      networkPolicy: true,
      alerts: true,
      statusPage: true,
      imageInventory: true,
    },
    ...p,
  };
}

describe("hasRemoteSyncError", () => {
  it("returns false when portal is undefined", () => {
    expect(hasRemoteSyncError(undefined)).toBe(false);
  });

  it("returns false when remoteSync is missing or lastSyncError is empty", () => {
    expect(hasRemoteSyncError(portal({ name: "p", title: "P" }))).toBe(false);
    expect(
      hasRemoteSyncError(
        portal({
          name: "p",
          title: "P",
          remoteSync: {
            lastSyncTime: "",
            lastSyncError: "",
            remoteTitle: "",
            fqdnCount: 0,
          },
        }),
      ),
    ).toBe(false);
    expect(
      hasRemoteSyncError(
        portal({
          name: "p",
          title: "P",
          remoteSync: {
            lastSyncTime: "",
            lastSyncError: "   ",
            remoteTitle: "",
            fqdnCount: 0,
          },
        }),
      ),
    ).toBe(false);
  });

  it("returns true when lastSyncError has content", () => {
    expect(
      hasRemoteSyncError(
        portal({
          name: "p",
          title: "P",
          isRemote: true,
          remoteSync: {
            lastSyncTime: "",
            lastSyncError: "connection refused",
            remoteTitle: "",
            fqdnCount: 0,
          },
        }),
      ),
    ).toBe(true);
  });
});

describe("portalRefForRoute", () => {
  it("returns metadata name when route matches subPath", () => {
    const portals = [
      portal({ name: "team-a", title: "Team A", subPath: "custom" }),
    ];
    expect(portalRefForRoute(portals, "custom")).toBe("team-a");
  });

  it("returns metadata name when route matches name and subPath is empty", () => {
    const portals = [portal({ name: "main", title: "Main", main: true })];
    expect(portalRefForRoute(portals, "main")).toBe("main");
  });

  it("falls back to segment when no portal matches", () => {
    expect(portalRefForRoute([], "unknown")).toBe("unknown");
  });
});
