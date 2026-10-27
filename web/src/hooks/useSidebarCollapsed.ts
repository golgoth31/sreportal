import { useCallback, useEffect, useState } from "react";

import { readStorage, writeStorage } from "@/lib/storage";

const STORAGE_KEY = "sreportal-sidebar-collapsed";

function getInitialCollapsed(): boolean {
  if (typeof window === "undefined") return false;
  return readStorage(STORAGE_KEY) === "true";
}

export function useSidebarCollapsed() {
  const [collapsed, setCollapsed] = useState<boolean>(getInitialCollapsed);

  useEffect(() => {
    writeStorage(STORAGE_KEY, collapsed ? "true" : "false");
  }, [collapsed]);

  // Keep other open tabs in sync
  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setCollapsed(e.newValue === "true");
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  const toggle = useCallback(() => {
    setCollapsed((prev) => !prev);
  }, []);

  return { collapsed, toggle };
}
