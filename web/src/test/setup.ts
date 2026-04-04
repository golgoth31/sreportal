import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll } from "vitest";

import { server } from "./msw/server";

/** Radix Tooltip / Popper uses ResizeObserver (not in jsdom). */
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

globalThis.ResizeObserver = ResizeObserverStub;

const originalGetBoundingClientRect =
  Element.prototype.getBoundingClientRect;

/**
 * happy-dom often reports 0×0; Recharts ResponsiveContainer then logs to stderr.
 * Keep real rects when the layout already has positive size.
 */
beforeAll(() => {
  Element.prototype.getBoundingClientRect = function (
    this: Element,
  ): DOMRect {
    const r = originalGetBoundingClientRect.call(this);
    if (r.width > 0 && r.height > 0) {
      return r;
    }
    return new DOMRect(r.x, r.y, 400, 225);
  };

  server.listen({ onUnhandledRequest: "error" });
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  Element.prototype.getBoundingClientRect = originalGetBoundingClientRect;
  server.close();
});
