import { createBrowserRouter, Navigate } from "react-router";

import { RootLayout } from "@/components/RootLayout";
import { McpPage } from "@/features/mcp/ui/McpPage";
import { LinksPage } from "@/pages/LinksPage";

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
        element: <LinksPage />,
      },
      {
        path: "help",
        element: <McpPage />,
      },
    ],
  },
]);
