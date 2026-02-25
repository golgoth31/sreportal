import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';

import { FqdnGroupComponent } from './fqdn-group.component';
import type { FqdnGroup } from '../../../../application/dns.facade';
import type { FQDN, OriginResourceRef } from '../../../../../gen/sreportal/v1/dns_pb';

// ─── Test helpers ────────────────────────────────────────────────────────────

function makeOriginRef(overrides: Partial<OriginResourceRef> = {}): OriginResourceRef {
  return {
    kind: 'service',
    namespace: 'production',
    name: 'api-svc',
    ...overrides,
  } as unknown as OriginResourceRef;
}

function makeFqdn(overrides: Partial<FQDN> = {}): FQDN {
  return {
    name: 'api.example.com',
    source: 'external-dns',
    groups: ['Services'],
    description: '',
    recordType: 'A',
    targets: ['10.0.0.1'],
    lastSeen: undefined,
    dnsResourceName: 'test-dns',
    dnsResourceNamespace: 'default',
    originRef: undefined,
    ...overrides,
  } as unknown as FQDN;
}

function makeGroup(fqdns: FQDN[], overrides: Partial<FqdnGroup> = {}): FqdnGroup {
  return {
    name: 'Services',
    source: 'external-dns',
    fqdns,
    ...overrides,
  };
}

// ─── Suite ───────────────────────────────────────────────────────────────────

describe('FqdnGroupComponent', () => {
  let fixture: ComponentFixture<FqdnGroupComponent>;

  function render(group: FqdnGroup): void {
    fixture = TestBed.createComponent(FqdnGroupComponent);
    fixture.componentRef.setInput('group', group);
    fixture.detectChanges();
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FqdnGroupComponent, NoopAnimationsModule],
    }).compileComponents();
  });

  // ── origin resource display ──────────────────────────────────────────────

  describe('origin resource display', () => {
    it('shows kind/namespace/name for an external-dns FQDN with originRef', () => {
      const group = makeGroup([
        makeFqdn({
          originRef: makeOriginRef({ kind: 'service', namespace: 'production', name: 'api-svc' }),
        }),
      ]);

      render(group);

      const originEl: HTMLElement | null = (fixture.nativeElement as HTMLElement)
        .querySelector('.fqdn-card__origin');
      expect(originEl).withContext('origin block should be rendered').toBeTruthy();
      expect(originEl!.textContent).toContain('service/production/api-svc');
    });

    it('shows the correct ref for an ingress resource', () => {
      const group = makeGroup([
        makeFqdn({
          originRef: makeOriginRef({ kind: 'ingress', namespace: 'default', name: 'web-ingress' }),
        }),
      ]);

      render(group);

      const originEl: HTMLElement | null = (fixture.nativeElement as HTMLElement)
        .querySelector('.fqdn-card__origin');
      expect(originEl!.textContent).toContain('ingress/default/web-ingress');
    });

    it('does not render origin block when originRef is absent', () => {
      const group = makeGroup([
        makeFqdn({ originRef: undefined }),
      ]);

      render(group);

      expect((fixture.nativeElement as HTMLElement).querySelector('.fqdn-card__origin'))
        .withContext('no origin block expected for entries without originRef')
        .toBeNull();
    });

    it('does not render origin block for manual entries (no originRef)', () => {
      const group = makeGroup(
        [makeFqdn({ originRef: undefined })],
        { source: 'manual' },
      );

      render(group);

      expect((fixture.nativeElement as HTMLElement).querySelector('.fqdn-card__origin'))
        .toBeNull();
    });
  });

  // ── group header ─────────────────────────────────────────────────────────

  describe('group header', () => {
    it('displays the group name', () => {
      render(makeGroup([makeFqdn()], { name: 'Production' }));
      expect((fixture.nativeElement as HTMLElement).textContent).toContain('Production');
    });

    it('shows "External DNS" label for external-dns source', () => {
      render(makeGroup([makeFqdn()], { source: 'external-dns' }));
      expect((fixture.nativeElement as HTMLElement).textContent).toContain('External DNS');
    });

    it('shows "Manual" label for manual source', () => {
      render(makeGroup([makeFqdn()], { source: 'manual' }));
      expect((fixture.nativeElement as HTMLElement).textContent).toContain('Manual');
    });

    it('displays the FQDN count', () => {
      const group = makeGroup([makeFqdn(), makeFqdn({ name: 'b.example.com' })]);
      render(group);
      expect((fixture.nativeElement as HTMLElement).textContent).toContain('2');
    });
  });

  // ── FQDN card ────────────────────────────────────────────────────────────

  describe('FQDN card', () => {
    it('renders the FQDN name as a link', () => {
      render(makeGroup([makeFqdn({ name: 'my.example.com' })]));
      const link: HTMLAnchorElement | null = (fixture.nativeElement as HTMLElement)
        .querySelector('a.fqdn-card__name');
      expect(link).toBeTruthy();
      expect(link!.textContent?.trim()).toBe('my.example.com');
    });
  });
});
