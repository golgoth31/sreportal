import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { http } from "msw";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { useDns } from "./useDns";
import {
  listFqdnsResponseJson,
  sampleFqdn,
} from "@/test/msw/connectJson";
import { grpcWebResponse, listFqdnsPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";

function createTestQueryWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return Wrapper;
}

describe("useDns", () => {
  it("when data is loaded exposes filtered and grouped views derived from domain helpers", async () => {
    server.use(
      http.post(listFqdnsPath, () =>
        grpcWebResponse(
          listFqdnsResponseJson([
            sampleFqdn({
              name: "a.example.com",
              groups: ["g1", "g2"],
              description: "Alpha",
            }),
            sampleFqdn({
              name: "b.example.com",
              groups: ["g2"],
              description: "Beta",
            }),
          ]),
        ),
      ),
    );

    const { result } = renderHook(() => useDns("main"), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.totalCount).toBe(2);
    expect(result.current.groups).toEqual(["g1", "g2"]);

    act(() => {
      result.current.setSearchTerm("alpha");
    });
    expect(result.current.filteredCount).toBe(1);
    expect(result.current.filtered[0]?.name).toBe("a.example.com");

    act(() => {
      result.current.setGroupFilter("g2");
    });
    expect(result.current.groupedByGroup.map((g) => g.name)).toEqual(["g2"]);
    expect(result.current.groupedByGroup[0]?.fqdns).toHaveLength(1);
    expect(result.current.groupedByGroup[0]?.fqdns[0]?.name).toBe(
      "a.example.com",
    );
  });
});
