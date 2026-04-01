import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

import { TruncatedCopyTooltipText } from "./TruncatedCopyTooltipText";

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const { copyMock } = vi.hoisted(() => ({
  copyMock: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@/hooks/useCopyToClipboard", () => ({
  useCopyToClipboard: (text: string) => ({
    copy: () => copyMock(text) as Promise<void>,
    copied: false,
  }),
}));

function wrapper({ children }: { children: React.ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("TruncatedCopyTooltipText", () => {
  beforeEach(() => {
    copyMock.mockClear();
  });

  afterEach(() => {
    cleanup();
  });

  it("when text is only whitespace renders nothing", () => {
    render(<TruncatedCopyTooltipText text="   " />, { wrapper });
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("when text is present shows copy button with default aria-label", () => {
    render(<TruncatedCopyTooltipText text="Hello" />, { wrapper });
    expect(
      screen.getByRole("button", { name: "Click to copy message" }),
    ).toHaveTextContent("Hello");
  });

  it("when enableCopy is false shows a focusable span with full text as aria-label", () => {
    render(
      <TruncatedCopyTooltipText text="team-a, team-b" enableCopy={false} />,
      { wrapper },
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    const inner = screen.getByText("team-a, team-b");
    const trigger = inner.closest("span[tabindex]");
    expect(trigger).toBeTruthy();
    expect(trigger).toHaveAttribute("aria-label", "team-a, team-b");
    expect(trigger).toHaveAttribute("tabIndex", "0");
  });

  it("when enableCopy is false click does not invoke copy", async () => {
    const user = userEvent.setup();
    render(
      <TruncatedCopyTooltipText text="receiver-one" enableCopy={false} />,
      { wrapper },
    );
    await user.click(screen.getByText("receiver-one"));
    expect(copyMock).not.toHaveBeenCalled();
  });

  it("when copyAriaLabel is set uses it for the button", () => {
    render(
      <TruncatedCopyTooltipText
        text="Alert summary"
        copyAriaLabel="Click to copy summary"
      />,
      { wrapper },
    );
    expect(
      screen.getByRole("button", { name: "Click to copy summary" }),
    ).toBeInTheDocument();
  });

  it("invokes copy with full text when the button is clicked", async () => {
    const user = userEvent.setup();
    const text = "Line one\nLine two";
    render(<TruncatedCopyTooltipText text={text} />, { wrapper });

    await user.click(
      screen.getByRole("button", { name: "Click to copy message" }),
    );

    expect(copyMock).toHaveBeenCalledWith(text);
  });
});
