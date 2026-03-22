import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { http } from "msw";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { listReleaseDaysResponseJson } from "@/test/msw/connectJson";
import { grpcWebResponse, listReleaseDaysPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";

import { useReleaseDays } from "./useReleaseDays";

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

describe("useReleaseDays", () => {
  it("returns the set of days and TTL days", async () => {
    server.use(
      http.post(listReleaseDaysPath, () =>
        grpcWebResponse(
          listReleaseDaysResponseJson(
            ["2026-03-19", "2026-03-20", "2026-03-21"],
            30,
          ),
        ),
      ),
    );

    const { result } = renderHook(() => useReleaseDays(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.daysSet).toEqual(
      new Set(["2026-03-19", "2026-03-20", "2026-03-21"]),
    );
    expect(result.current.ttlDays).toBe(30);
  });

  it("returns empty set when no releases exist", async () => {
    server.use(
      http.post(listReleaseDaysPath, () =>
        grpcWebResponse(listReleaseDaysResponseJson([], 30)),
      ),
    );

    const { result } = renderHook(() => useReleaseDays(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.daysSet).toEqual(new Set());
    expect(result.current.ttlDays).toBe(30);
  });

  it("computes isDayDisabled correctly", async () => {
    server.use(
      http.post(listReleaseDaysPath, () =>
        grpcWebResponse(
          listReleaseDaysResponseJson(["2026-03-21"], 30),
        ),
      ),
    );

    const { result } = renderHook(() => useReleaseDays(), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // Day with releases — not disabled
    expect(result.current.isDayDisabled(new Date("2026-03-21"))).toBe(false);
    // Day without releases — disabled
    expect(result.current.isDayDisabled(new Date("2026-03-22"))).toBe(true);
  });
});
