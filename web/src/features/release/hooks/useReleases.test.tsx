import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { http } from "msw";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import {
  listReleasesResponseJson,
  sampleReleaseEntry,
} from "@/test/msw/connectJson";
import { grpcWebResponse, listReleasesPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";

import { useReleases } from "./useReleases";

function createTestQueryWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return Wrapper;
}

describe("useReleases", () => {
  it("when data is loaded returns entries for the day", async () => {
    server.use(
      http.post(listReleasesPath, () =>
        grpcWebResponse(
          listReleasesResponseJson("2026-03-21", [
            sampleReleaseEntry({
              type: "deployment",
              version: "v1.0.0",
              origin: "ci/cd",
              author: "alice",
              message: "fix login",
            }),
            sampleReleaseEntry({
              type: "rollback",
              version: "v0.9.0",
              origin: "manual",
            }),
          ]),
        ),
      ),
    );

    const { result } = renderHook(() => useReleases(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.day).toBe("2026-03-21");
    expect(result.current.entries).toHaveLength(2);
    expect(result.current.totalCount).toBe(2);
  });

  it("filters entries by keyword search", async () => {
    server.use(
      http.post(listReleasesPath, () =>
        grpcWebResponse(
          listReleasesResponseJson("2026-03-21", [
            sampleReleaseEntry({
              type: "deployment",
              version: "v1.0.0",
              origin: "ci/cd",
              message: "fix login",
            }),
            sampleReleaseEntry({
              type: "rollback",
              version: "v0.9.0",
              origin: "manual",
            }),
          ]),
        ),
      ),
    );

    const { result } = renderHook(() => useReleases(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => {
      result.current.setSearch("rollback");
    });

    expect(result.current.entries).toHaveLength(1);
    expect(result.current.entries[0]?.type).toBe("rollback");
    expect(result.current.hasFilters).toBe(true);
  });

  it("exposes day navigation fields", async () => {
    server.use(
      http.post(listReleasesPath, () =>
        grpcWebResponse(
          listReleasesResponseJson(
            "2026-03-21",
            [sampleReleaseEntry({ type: "deploy", version: "v1", origin: "ci" })],
            "2026-03-20",
            "2026-03-22",
          ),
        ),
      ),
    );

    const { result } = renderHook(() => useReleases(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.previousDay).toBe("2026-03-20");
    expect(result.current.nextDay).toBe("2026-03-22");
  });
});
