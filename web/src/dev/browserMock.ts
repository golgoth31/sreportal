/**
 * Wires MSW into the running browser when `VITE_MOCK=1`. No-op otherwise.
 */
import { setupWorker } from "msw/browser";

import { devHandlers } from "./mockHandlers";

export async function startBrowserMocks(): Promise<void> {
  const worker = setupWorker(...devHandlers);
  await worker.start({
    onUnhandledRequest: "bypass",
    quiet: false,
  });
}
