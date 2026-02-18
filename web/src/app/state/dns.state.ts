import { Injectable, signal, computed } from '@angular/core';
import type { FQDN } from '../../gen/sreportal/v1/dns_pb';

@Injectable({
  providedIn: 'root'
})
export class DnsState {
  // Private signals for state
  private readonly _fqdns = signal<FQDN[]>([]);
  private readonly _loading = signal<boolean>(false);
  private readonly _error = signal<string | null>(null);
  private readonly _searchTerm = signal<string>('');
  private readonly _groupFilter = signal<string>('');

  // Public readonly signals
  readonly fqdns = this._fqdns.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();
  readonly searchTerm = this._searchTerm.asReadonly();
  readonly groupFilter = this._groupFilter.asReadonly();

  // Computed signals
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

  readonly groupedByGroup = computed(() => {
    const fqdns = this.filteredFqdns();
    const groups = new Map<string, FQDN[]>();

    for (const fqdn of fqdns) {
      const fqdnGroups = fqdn.groups.length > 0 ? fqdn.groups : ['Ungrouped'];
      for (const group of fqdnGroups) {
        if (!groups.has(group)) {
          groups.set(group, []);
        }
        groups.get(group)!.push(fqdn);
      }
    }

    return groups;
  });

  readonly totalCount = computed(() => this._fqdns().length);
  readonly filteredCount = computed(() => this.filteredFqdns().length);

  readonly groups = computed(() => {
    const fqdns = this._fqdns();
    const groups = new Set<string>();
    for (const fqdn of fqdns) {
      for (const group of fqdn.groups) {
        groups.add(group);
      }
    }
    return Array.from(groups).sort();
  });

  readonly sources = computed(() => {
    const fqdns = this._fqdns();
    const sources = new Set<string>();
    for (const fqdn of fqdns) {
      if (fqdn.source) {
        sources.add(fqdn.source);
      }
    }
    return Array.from(sources).sort();
  });

  // Actions
  setFqdns(fqdns: FQDN[]): void {
    this._fqdns.set(fqdns);
    this._error.set(null);
  }

  addFqdn(fqdn: FQDN): void {
    this._fqdns.update(current => [...current, fqdn]);
  }

  updateFqdn(fqdn: FQDN): void {
    this._fqdns.update(current =>
      current.map(f => f.name === fqdn.name ? fqdn : f)
    );
  }

  removeFqdn(name: string): void {
    this._fqdns.update(current =>
      current.filter(f => f.name !== name)
    );
  }

  setLoading(loading: boolean): void {
    this._loading.set(loading);
  }

  setError(error: string | null): void {
    this._error.set(error);
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
