import { DestroyRef, Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DnsService } from '../services/dns.service';
import type { FQDN } from '../../gen/sreportal/v1/dns_pb';

export interface FqdnGroup {
  readonly name: string;
  readonly source: string;
  readonly fqdns: readonly FQDN[];
}

@Injectable()
export class DnsFacade {
  private readonly dnsService = inject(DnsService);
  private readonly destroyRef = inject(DestroyRef);

  // Private writable signals
  private readonly _fqdns = signal<FQDN[]>([]);
  private readonly _loading = signal(false);
  private readonly _error = signal<string | null>(null);
  private readonly _searchTerm = signal('');
  private readonly _groupFilter = signal('');

  // Public read-only signals
  readonly fqdns = this._fqdns.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();
  readonly searchTerm = this._searchTerm.asReadonly();
  readonly groupFilter = this._groupFilter.asReadonly();

  // Derived state
  readonly filteredFqdns = computed(() => {
    const fqdns = this._fqdns();
    const search = this._searchTerm().toLowerCase();
    const group = this._groupFilter();

    return fqdns.filter(fqdn => {
      const matchesSearch = !search || fqdn.name.toLowerCase().includes(search);
      const matchesGroup = !group || fqdn.groups.includes(group);
      return matchesSearch && matchesGroup;
    });
  });

  readonly groupedByGroup = computed((): FqdnGroup[] => {
    const grouped = new Map<string, { source: string; fqdns: FQDN[] }>();

    for (const fqdn of this.filteredFqdns()) {
      const groupNames = fqdn.groups.length > 0 ? fqdn.groups : ['Ungrouped'];
      for (const name of groupNames) {
        const existing = grouped.get(name);
        if (existing) {
          existing.fqdns.push(fqdn);
        } else {
          grouped.set(name, { source: fqdn.source, fqdns: [fqdn] });
        }
      }
    }

    return Array.from(grouped.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([name, { source, fqdns }]) => ({ name, source, fqdns }));
  });

  readonly totalCount = computed(() => this._fqdns().length);
  readonly filteredCount = computed(() => this.filteredFqdns().length);

  readonly groups = computed(() => {
    const groups = new Set<string>();
    for (const fqdn of this._fqdns()) {
      for (const group of fqdn.groups) {
        groups.add(group);
      }
    }
    return Array.from(groups).sort();
  });

  // Actions
  loadFqdns(portalName: string): void {
    this._loading.set(true);
    this._error.set(null);

    this.dnsService.listFQDNs(portalName)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: fqdns => {
          this._fqdns.set(fqdns);
          this._loading.set(false);
        },
        error: (err: Error) => {
          this._error.set(err.message);
          this._loading.set(false);
        },
      });
  }

  setSearchTerm(term: string): void {
    this._searchTerm.set(term);
  }

  setGroupFilter(group: string): void {
    this._groupFilter.set(group);
  }

  clearFilters(): void {
    this._searchTerm.set('');
    this._groupFilter.set('');
  }
}
