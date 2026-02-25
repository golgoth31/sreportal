import { Injectable, signal, computed, effect } from '@angular/core';

export type ThemeMode = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'sreportal-theme';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly _mode = signal<ThemeMode>(this.loadStoredMode());

  readonly mode = this._mode.asReadonly();

  readonly isDark = computed(() => {
    const m = this._mode();
    if (m === 'system') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    return m === 'dark';
  });

  readonly icon = computed(() => {
    switch (this._mode()) {
      case 'light': return 'light_mode';
      case 'dark': return 'dark_mode';
      case 'system': return 'contrast';
    }
  });

  constructor() {
    effect(() => {
      const dark = this.isDark();
      document.documentElement.classList.toggle('dark', dark);
    });

    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (this._mode() === 'system') {
        document.documentElement.classList.toggle(
          'dark',
          window.matchMedia('(prefers-color-scheme: dark)').matches,
        );
      }
    });
  }

  toggle(): void {
    const order: ThemeMode[] = ['light', 'dark', 'system'];
    const current = order.indexOf(this._mode());
    const next = order[(current + 1) % order.length];
    this._mode.set(next);
    localStorage.setItem(STORAGE_KEY, next);
  }

  private loadStoredMode(): ThemeMode {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored;
    }
    return 'system';
  }
}
