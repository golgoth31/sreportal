import { lazy, Suspense } from "react";
import { createBrowserRouter, Navigate } from "react-router";

import { RootLayout } from "@/components/RootLayout";

const LinksPage = lazy(() =>
  import("@/pages/LinksPage").then((m) => ({ default: m.LinksPage }))
);
const McpPage = lazy(() =>
  import("@/features/mcp/ui/McpPage").then((m) => ({ default: m.McpPage }))
);

export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootLayout />,
    children: [
      {
        index: true,
        element: <Navigate to="/main/links" replace />,
      },
      {
        path: ":portalName/links",
        element: (
          <Suspense fallback={null}>
            <LinksPage />
          </Suspense>
        ),
      },
      {
        path: "help",
        element: (
          <Suspense fallback={null}>
            <McpPage />
          </Suspense>
        ),
      },
    ],
  },
]);
