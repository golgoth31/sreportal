import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { AlertGroup } from "../domain/alertmanager.types";
import { AlertGroupCard } from "./AlertGroupCard";

const group: AlertGroup = {
  name: "Watchdog",
  alerts: [
    {
      fingerprint: "fp1",
      labels: { alertname: "Watchdog", instance: "host:9090" },
      annotations: { summary: "all good" },
      state: "active",
      startsAt: new Date("2026-01-01T00:00:00Z").toISOString(),
      endsAt: undefined,
      updatedAt: new Date("2026-01-01T00:00:00Z").toISOString(),
      receivers: ["default"],
      silencedBy: [],
    },
  ],
};

describe("AlertGroupCard Soft Material density", () => {
  it("uses mono for receivers, summary, and instance", async () => {
    const user = userEvent.setup();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <AlertGroupCard group={group} />
      </QueryClientProvider>,
    );
    await user.click(screen.getByRole("button", { name: /Watchdog/i }));

    // Truncated triggers carry the class on the interactive element (aria-label).
    expect(screen.getByLabelText("default").className).toMatch(/font-mono/);
    expect(screen.getByLabelText("Click to copy summary").className).toMatch(
      /font-mono/,
    );
    expect(screen.getByText("host:9090").className).toMatch(/font-mono/);
  });
});
