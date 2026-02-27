import { useCallback, useEffect, useState } from "react";

export type ThemeMode = "light" | "dark" | "system";

const STORAGE_KEY = "sreportal-theme";

function getInitialMode(): ThemeMode {
  if (typeof window === "undefined") return "system";
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark" || stored === "system") {
    return stored;
  }
  return "system";
}

function applyTheme(mode: ThemeMode) {
  const isDark =
    mode === "dark" ||
    (mode === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  document.documentElement.classList.toggle("dark", isDark);
}

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(getInitialMode);

  useEffect(() => {
    applyTheme(mode);
    localStorage.setItem(STORAGE_KEY, mode);
  }, [mode]);

  // Listen for system preference changes when in "system" mode
  useEffect(() => {
    if (mode !== "system") return;

    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => applyTheme("system");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [mode]);

  const toggle = useCallback(() => {
    setMode((prev) => {
      if (prev === "light") return "dark";
      if (prev === "dark") return "system";
      return "light";
    });
  }, []);

  return { mode, toggle };
}
