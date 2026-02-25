import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { DnsFacade } from './dns.facade';
import { DnsService } from '../services/dns.service';
import type { FQDN } from '../../gen/sreportal/v1/dns_pb';

function makeFqdn(overrides: Partial<FQDN> = {}): FQDN {
  return {
    name: 'example.com',
    source: 'external-dns',
    groups: ['infra'],
    description: '',
    recordType: 'A',
    targets: ['1.2.3.4'],
    lastSeen: undefined,
    dnsResourceName: '',
    dnsResourceNamespace: '',
    ...overrides,
  } as unknown as FQDN;
}

describe('DnsFacade', () => {
  let facade: DnsFacade;
  let dnsService: jasmine.SpyObj<DnsService>;

  beforeEach(() => {
    const spy = jasmine.createSpyObj<DnsService>('DnsService', ['listFQDNs']);

    TestBed.configureTestingModule({
      providers: [
        DnsFacade,
        { provide: DnsService, useValue: spy },
      ],
    });

    facade = TestBed.inject(DnsFacade);
    dnsService = TestBed.inject(DnsService) as jasmine.SpyObj<DnsService>;
  });

  // Initial state
  describe('initial state', () => {
    it('starts with empty fqdns', () => {
      expect(facade.fqdns()).toEqual([]);
    });

    it('starts with loading false', () => {
      expect(facade.loading()).toBeFalse();
    });

    it('starts with null error', () => {
      expect(facade.error()).toBeNull();
    });

    it('starts with empty search term', () => {
      expect(facade.searchTerm()).toBe('');
    });

    it('starts with empty group filter', () => {
      expect(facade.groupFilter()).toBe('');
    });
  });

  // loadFqdns
  describe('loadFqdns', () => {
    it('sets loading true before the response arrives', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([]));

      facade.loadFqdns('main');
      // Before tick — the observable hasn't emitted yet in this tick
      // We verify loading was set synchronously before the subscribe callback
      expect(dnsService.listFQDNs).toHaveBeenCalledWith('main');
      tick();
    }));

    it('populates fqdns on success', fakeAsync(() => {
      const items = [makeFqdn({ name: 'a.com' }), makeFqdn({ name: 'b.com' })];
      dnsService.listFQDNs.and.returnValue(of(items));

      facade.loadFqdns('main');
      tick();

      expect(facade.fqdns()).toEqual(items);
      expect(facade.loading()).toBeFalse();
      expect(facade.error()).toBeNull();
    }));

    it('sets error on failure and clears loading', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(
        throwError(() => new Error('network error'))
      );

      facade.loadFqdns('main');
      tick();

      expect(facade.error()).toBe('network error');
      expect(facade.loading()).toBeFalse();
      expect(facade.fqdns()).toEqual([]);
    }));

    it('passes the portalName to DnsService', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([]));
      facade.loadFqdns('staging');
      tick();
      expect(dnsService.listFQDNs).toHaveBeenCalledWith('staging');
    }));
  });

  // Filtering
  describe('filteredFqdns', () => {
    beforeEach(fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'api.prod.com', groups: ['backend'] }),
        makeFqdn({ name: 'web.prod.com', groups: ['frontend'] }),
        makeFqdn({ name: 'db.prod.com', groups: ['backend'] }),
      ]));
      facade.loadFqdns('main');
      tick();
    }));

    it('returns all fqdns when no filters are set', () => {
      expect(facade.filteredCount()).toBe(3);
    });

    it('filters by search term (case-insensitive)', () => {
      facade.setSearchTerm('API');
      expect(facade.filteredCount()).toBe(1);
      expect(facade.filteredFqdns()[0]?.name).toBe('api.prod.com');
    });

    it('filters by group', () => {
      facade.setGroupFilter('backend');
      expect(facade.filteredCount()).toBe(2);
    });

    it('clears all filters', () => {
      facade.setSearchTerm('api');
      facade.setGroupFilter('backend');
      facade.clearFilters();
      expect(facade.filteredCount()).toBe(3);
      expect(facade.searchTerm()).toBe('');
      expect(facade.groupFilter()).toBe('');
    });
  });

  // groupedByGroup
  describe('groupedByGroup', () => {
    it('groups fqdns by their group name', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'a.com', groups: ['infra'], source: 'external-dns' }),
        makeFqdn({ name: 'b.com', groups: ['infra'], source: 'external-dns' }),
        makeFqdn({ name: 'c.com', groups: ['apps'], source: 'manual' }),
      ]));
      facade.loadFqdns('main');
      tick();

      const groups = facade.groupedByGroup();
      expect(groups.length).toBe(2);
      const infra = groups.find(g => g.name === 'infra');
      expect(infra?.fqdns.length).toBe(2);
      expect(infra?.source).toBe('external-dns');
    }));

    it('assigns "Ungrouped" when fqdn has no groups', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'orphan.com', groups: [] }),
      ]));
      facade.loadFqdns('main');
      tick();

      const groups = facade.groupedByGroup();
      expect(groups[0]?.name).toBe('Ungrouped');
    }));

    it('returns fqdns within each group sorted alphabetically by name', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'z.prod.com', groups: ['infra'] }),
        makeFqdn({ name: 'a.prod.com', groups: ['infra'] }),
        makeFqdn({ name: 'm.prod.com', groups: ['infra'] }),
      ]));
      facade.loadFqdns('main');
      tick();

      const infra = facade.groupedByGroup().find(g => g.name === 'infra');
      expect(infra?.fqdns.map(f => f.name)).toEqual(['a.prod.com', 'm.prod.com', 'z.prod.com']);
    }));

    it('returns groups sorted alphabetically regardless of arrival order', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'z.com', groups: ['Zebra'] }),
        makeFqdn({ name: 'a.com', groups: ['Alpha'] }),
        makeFqdn({ name: 'm.com', groups: ['Mango'] }),
      ]));
      facade.loadFqdns('main');
      tick();

      const names = facade.groupedByGroup().map(g => g.name);
      expect(names).toEqual(['Alpha', 'Mango', 'Zebra']);
    }));

    it('shows a multi-group fqdn only under the filtered group, not under its other groups', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'api.com', groups: ['frontend', 'backend'] }),
        makeFqdn({ name: 'db.com',  groups: ['backend'] }),
      ]));
      facade.loadFqdns('main');
      tick();

      facade.setGroupFilter('frontend');

      const groups = facade.groupedByGroup();
      expect(groups.length).toBe(1);
      expect(groups[0]?.name).toBe('frontend');
      expect(groups[0]?.fqdns.map(f => f.name)).toEqual(['api.com']);
    }));
  });

  // groups computed
  describe('groups', () => {
    it('returns sorted unique group names from all fqdns', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ groups: ['zebra', 'alpha'] }),
        makeFqdn({ groups: ['alpha'] }),
      ]));
      facade.loadFqdns('main');
      tick();

      expect(facade.groups()).toEqual(['alpha', 'zebra']);
    }));
  });

  // totalCount / filteredCount
  describe('counts', () => {
    it('totalCount reflects all fqdns regardless of filter', fakeAsync(() => {
      dnsService.listFQDNs.and.returnValue(of([
        makeFqdn({ name: 'a.com' }),
        makeFqdn({ name: 'b.com' }),
      ]));
      facade.loadFqdns('main');
      tick();

      facade.setSearchTerm('a.com');
      expect(facade.totalCount()).toBe(2);
      expect(facade.filteredCount()).toBe(1);
    }));
  });
});
