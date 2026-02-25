import { Injectable, signal, computed, effect } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { fromEvent, map } from 'rxjs';

export type ThemeMode = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'sreportal-theme';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly _mode = signal<ThemeMode>(this.loadStoredMode());

  private readonly mq = window.matchMedia('(prefers-color-scheme: dark)');

  // toSignal subscribes and auto-cleans up via DestroyRef — no manual listener needed
  private readonly _systemDark = toSignal(
    fromEvent<MediaQueryListEvent>(this.mq, 'change').pipe(map(e => e.matches)),
    { initialValue: this.mq.matches },
  );

  readonly mode = this._mode.asReadonly();

  readonly isDark = computed(() => {
    const m = this._mode();
    if (m === 'system') return this._systemDark();
    return m === 'dark';
  });

  readonly icon = computed(() => {
    switch (this._mode()) {
      case 'light':  return 'light_mode';
      case 'dark':   return 'dark_mode';
      case 'system': return 'contrast';
    }
  });

  constructor() {
    effect(() => {
      document.documentElement.classList.toggle('dark', this.isDark());
    });
  }

  toggle(): void {
    const order: ThemeMode[] = ['light', 'dark', 'system'];
    const next = order[(order.indexOf(this._mode()) + 1) % order.length] ?? 'light';
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
