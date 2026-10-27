// localStorage access can throw (Safari private mode, blocked site data,
// sandboxed iframes). These helpers degrade to "no persisted value" instead.

export function readStorage(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeStorage(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Persistence is best-effort.
  }
}
