import { create } from "@bufbuild/protobuf";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { http } from "msw";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { DeployStatusCommitSchema } from "@/gen/sreportal/v1/deploystatus_pb";
import {
  listDeployStatusResponseJson,
  sampleDeployStatusEntry,
} from "@/test/msw/connectJson";
import { grpcWebResponse, listDeployStatusPath } from "@/test/msw/handlers";
import { server } from "@/test/msw/server";
import { useDeployStatusQuery } from "./useDeployStatusQuery";

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

describe("useDeployStatusQuery", () => {
  it("returns entries from the API — one behind with pending commits, one ok", async () => {
    server.use(
      http.post(listDeployStatusPath, () =>
        grpcWebResponse(
          listDeployStatusResponseJson([
            sampleDeployStatusEntry({
              key: "default/api-server",
              state: "behind",
              image: "ghcr.io/example/api-server:abc1234",
              aheadBy: 2,
              deployRunUrl: "https://github.com/example/api-server/actions/runs/1",
              pendingCommits: [
                create(DeployStatusCommitSchema, {
                  sha: "deadbeef",
                  message: "fix: correct auth header",
                  author: "alice",
                  url: "https://github.com/example/api-server/commit/deadbeef",
                }),
              ],
            }),
            sampleDeployStatusEntry({
              key: "default/frontend",
              state: "ok",
              image: "ghcr.io/example/frontend:xyz9876",
            }),
          ]),
        ),
      ),
    );

    const { result } = renderHook(() => useDeployStatusQuery("main"), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.entries).toHaveLength(2);

    const behind = result.current.entries.find((e) => e.key === "default/api-server");
    expect(behind?.state).toBe("behind");
    expect(behind?.aheadBy).toBe(2);
    expect(behind?.pendingCommits).toHaveLength(1);
    expect(behind?.pendingCommits[0]?.message).toBe("fix: correct auth header");
    expect(behind?.deployRunUrl).toBe(
      "https://github.com/example/api-server/actions/runs/1",
    );

    const ok = result.current.entries.find((e) => e.key === "default/frontend");
    expect(ok?.state).toBe("ok");
    expect(ok?.image).toBe("ghcr.io/example/frontend:xyz9876");
  });

  it("returns empty array when API returns no entries", async () => {
    server.use(
      http.post(listDeployStatusPath, () =>
        grpcWebResponse(listDeployStatusResponseJson([])),
      ),
    );

    const { result } = renderHook(() => useDeployStatusQuery("main"), {
      wrapper: createTestQueryWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.entries).toHaveLength(0);
    expect(result.current.error).toBeNull();
  });
});
