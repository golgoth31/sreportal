import { render, screen } from "@testing-library/react";
import { http } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, it, expect } from "vitest";

import { server } from "@/test/msw/server";
import {
  grpcWebResponse,
  listCustomEmojisPath,
} from "@/test/msw/handlers";
import { listCustomEmojisResponseJson } from "@/test/msw/connectJson";
import { EmojiText } from "./EmojiText";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("EmojiText", () => {
  it("renders plain text without shortcodes unchanged", () => {
    render(<EmojiText text="Hello world" />, { wrapper: createWrapper() });
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });

  it("resolves standard emoji shortcodes to unicode", () => {
    render(<EmojiText text="Deploy :rocket: done" />, {
      wrapper: createWrapper(),
    });
    expect(screen.getByRole("img", { name: "rocket" })).toHaveTextContent(
      "🚀",
    );
    expect(screen.getByText(/Deploy/)).toBeInTheDocument();
    expect(screen.getByText(/done/)).toBeInTheDocument();
  });

  it("resolves custom Slack emojis to img tags", async () => {
    server.use(
      http.post(listCustomEmojisPath, () =>
        grpcWebResponse(
          listCustomEmojisResponseJson({
            rabbitmq: "https://emoji.slack.com/rabbitmq.png",
          }),
        ),
      ),
    );

    render(<EmojiText text="Queue :rabbitmq: ready" />, {
      wrapper: createWrapper(),
    });

    const img = await screen.findByAltText(":rabbitmq:");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute(
      "src",
      "https://emoji.slack.com/rabbitmq.png",
    );
  });

  it("leaves unknown shortcodes as-is", () => {
    render(<EmojiText text="Status :unknown_emoji_xyz:" />, {
      wrapper: createWrapper(),
    });
    expect(screen.getByText(/Status :unknown_emoji_xyz:/)).toBeInTheDocument();
  });

  it("handles empty text", () => {
    const { container } = render(<EmojiText text="" />, {
      wrapper: createWrapper(),
    });
    expect(container.querySelector("span")).toHaveTextContent("");
  });

  it("handles multiple shortcodes in one text", () => {
    const { container } = render(
      <EmojiText text=":rocket: and :warning:" />,
      { wrapper: createWrapper() },
    );
    const imgs = container.querySelectorAll("[role='img']");
    expect(imgs).toHaveLength(2);
    expect(imgs[0]).toHaveAttribute("aria-label", "rocket");
    expect(imgs[0]).toHaveTextContent("🚀");
    expect(imgs[1]).toHaveAttribute("aria-label", "warning");
  });
});
