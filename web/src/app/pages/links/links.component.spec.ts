import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { signal } from '@angular/core';
import { of } from 'rxjs';

import { LinksComponent } from './links.component';
import { DnsFacade } from '../../application/dns.facade';
import { PortalServiceClient } from '../../services/portal.service';
import type { Portal } from '../../../gen/sreportal/v1/portal_pb';
import type { FqdnGroup } from '../../application/dns.facade';

// ── Helpers ───────────────────────────────────────────────────────────────────

function makePortal(overrides: Partial<Portal> = {}): Portal {
  return {
    name: 'main',
    title: 'Main',
    subPath: 'main',
    main: true,
    namespace: 'default',
    ready: true,
    isRemote: false,
    url: '',
    ...overrides,
  } as unknown as Portal;
}

function makeFacadeMock(): DnsFacade {
  return {
    loading:       signal(false),
    error:         signal<string | null>(null),
    searchTerm:    signal(''),
    groupFilter:   signal(''),
    groups:        signal<string[]>([]),
    filteredCount: signal(0),
    totalCount:    signal(0),
    groupedByGroup: signal<FqdnGroup[]>([]),
    loadFqdns:     jasmine.createSpy('loadFqdns'),
    setSearchTerm: jasmine.createSpy('setSearchTerm'),
    setGroupFilter: jasmine.createSpy('setGroupFilter'),
    clearFilters:  jasmine.createSpy('clearFilters'),
  } as unknown as DnsFacade;
}

// ── Suite ─────────────────────────────────────────────────────────────────────

describe('LinksComponent remote portal button', () => {
  let fixture: ComponentFixture<LinksComponent>;

  function setupModule(portals: Portal[], portalName: string): void {
    const portalSpy = jasmine.createSpyObj<PortalServiceClient>(
      'PortalServiceClient',
      ['listPortals'],
    );
    portalSpy.listPortals.and.returnValue(of(portals));

    TestBed.configureTestingModule({
      imports: [LinksComponent, NoopAnimationsModule],
      providers: [
        { provide: PortalServiceClient, useValue: portalSpy },
        { provide: ActivatedRoute, useValue: { params: of({ portalName }) } },
      ],
    });

    // Replace the component-scoped DnsFacade provider with a lightweight mock
    TestBed.overrideComponent(LinksComponent, {
      set: { providers: [{ provide: DnsFacade, useValue: makeFacadeMock() }] },
    });
  }

  // ── Remote portal ────────────────────────────────────────────────────────────

  describe('when the current portal is remote', () => {
    beforeEach(async () => {
      setupModule(
        [makePortal({
          name: 'staging', subPath: 'staging',
          isRemote: true, url: 'https://staging.example.com', main: false,
        })],
        'staging',
      );
      await TestBed.compileComponents();
      fixture = TestBed.createComponent(LinksComponent);
      fixture.detectChanges();
    });

    it('shows the remote portal link button', fakeAsync(() => {
      tick();
      fixture.detectChanges();

      const link = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLAnchorElement>('[data-testid="remote-portal-link"]');
      expect(link).withContext('remote portal link should be present').toBeTruthy();
    }));

    it('links to the remote portal URL', fakeAsync(() => {
      tick();
      fixture.detectChanges();

      const link = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLAnchorElement>('[data-testid="remote-portal-link"]');
      expect(link?.href).toContain('https://staging.example.com');
    }));

    it('opens in a new tab', fakeAsync(() => {
      tick();
      fixture.detectChanges();

      const link = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLAnchorElement>('[data-testid="remote-portal-link"]');
      expect(link?.target).toBe('_blank');
    }));

    it('has rel="noopener noreferrer" for security', fakeAsync(() => {
      tick();
      fixture.detectChanges();

      const link = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLAnchorElement>('[data-testid="remote-portal-link"]');
      expect(link?.rel).toContain('noopener');
      expect(link?.rel).toContain('noreferrer');
    }));
  });

  // ── Local portal ─────────────────────────────────────────────────────────────

  describe('when the current portal is local', () => {
    beforeEach(async () => {
      setupModule(
        [makePortal({ name: 'main', subPath: 'main', isRemote: false, url: '' })],
        'main',
      );
      await TestBed.compileComponents();
      fixture = TestBed.createComponent(LinksComponent);
      fixture.detectChanges();
    });

    it('does not show the remote portal link button', fakeAsync(() => {
      tick();
      fixture.detectChanges();

      const link = (fixture.nativeElement as HTMLElement)
        .querySelector('[data-testid="remote-portal-link"]');
      expect(link).toBeNull();
    }));
  });
});
