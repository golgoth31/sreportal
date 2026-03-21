import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RemoteSyncStaleBanner } from "./RemoteSyncStaleBanner";

describe("RemoteSyncStaleBanner", () => {
  it("when lastSyncError is non-empty displays warning role and error detail", () => {
    render(
      <RemoteSyncStaleBanner lastSyncError="dial tcp: connection refused" />,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.getByText(/synchronization failed/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/may not reflect the current state/i),
    ).toBeInTheDocument();
  });

  it("when lastSyncError is whitespace-only renders nothing", () => {
    const { container } = render(<RemoteSyncStaleBanner lastSyncError="   " />);
    expect(container.firstChild).toBeNull();
  });
});
