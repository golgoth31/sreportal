import { AlertCircleIcon } from "lucide-react";
import { lazy, Suspense } from "react";
import { createBrowserRouter, Navigate, useRouteError } from "react-router";

import { Skeleton } from "@/components/ui/skeleton";
import { RootLayout } from "@/components/RootLayout";

function PageSkeleton() {
  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-4">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-14 w-full rounded-lg" />
      <Skeleton className="h-14 w-full rounded-lg" />
      <Skeleton className="h-14 w-full rounded-lg" />
    </div>
  );
}

function ErrorPage() {
  const error = useRouteError();
  const message = error instanceof Error ? error.message : String(error);
  return (
    <div className="max-w-screen-xl mx-auto px-4 py-16 flex flex-col items-center gap-4 text-center">
      <AlertCircleIcon className="size-10 text-destructive" />
      <h1 className="text-xl font-semibold">Something went wrong</h1>
      <p className="text-muted-foreground text-sm">{message}</p>
    </div>
  );
}

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
    errorElement: <ErrorPage />,
    children: [
      {
        index: true,
        element: <Navigate to="/main/links" replace />,
      },
      {
        path: ":portalName/links",
        errorElement: <ErrorPage />,
        element: (
          <Suspense fallback={<PageSkeleton />}>
            <LinksPage />
          </Suspense>
        ),
      },
      {
        path: "help",
        errorElement: <ErrorPage />,
        element: (
          <Suspense fallback={<PageSkeleton />}>
            <McpPage />
          </Suspense>
        ),
      },
    ],
  },
]);
