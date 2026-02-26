import { Component } from '@angular/core';
import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { of } from 'rxjs';

import { AppComponent } from './app.component';
import { PortalServiceClient } from './services/portal.service';
import type { Portal } from '../gen/sreportal/v1/portal_pb';

// ── Stubs ─────────────────────────────────────────────────────────────────────

@Component({ standalone: true, template: '' })
class StubPageComponent {}

function makePortal(name: string, title: string, subPath: string): Portal {
  return { name, title, subPath } as unknown as Portal;
}

// ── Suite ─────────────────────────────────────────────────────────────────────

describe('AppComponent toolbar navigation', () => {
  let fixture: ComponentFixture<AppComponent>;

  function setup(portals: Portal[]): void {
    const portalSpy = jasmine.createSpyObj<PortalServiceClient>(
      'PortalServiceClient',
      ['listPortals'],
    );
    portalSpy.listPortals.and.returnValue(of(portals));

    TestBed.configureTestingModule({
      imports: [AppComponent, NoopAnimationsModule],
      providers: [
        provideRouter([
          { path: ':portalName/links', component: StubPageComponent },
          { path: 'help',             component: StubPageComponent },
        ]),
        { provide: PortalServiceClient, useValue: portalSpy },
      ],
    });
  }

  // ── Rendering ───────────────────────────────────────────────────────────────

  describe('rendering', () => {
    beforeEach(fakeAsync(() => {
      setup([
        makePortal('main',    'Main Portal', 'main'),
        makePortal('staging', 'Staging',     'staging'),
      ]);
      fixture = TestBed.createComponent(AppComponent);
      fixture.detectChanges();
      tick();
      fixture.detectChanges();
    }));

    it('renders a nav link per portal plus the Help link', () => {
      const links = (fixture.nativeElement as HTMLElement)
        .querySelectorAll('a.toolbar__link');
      expect(links.length).toBe(3); // 2 portals + Help
    });

    it('no link has aria-current before any navigation', () => {
      const links = Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll('a.toolbar__link'),
      );
      for (const link of links) {
        expect(link.getAttribute('aria-current'))
          .withContext(`${link.textContent?.trim()} should have no aria-current`)
          .toBeNull();
      }
    });
  });

  // ── aria-current on active route ────────────────────────────────────────────

  describe('aria-current', () => {
    beforeEach(fakeAsync(() => {
      setup([
        makePortal('main',    'Main Portal', 'main'),
        makePortal('staging', 'Staging',     'staging'),
      ]);
      fixture = TestBed.createComponent(AppComponent);
      fixture.detectChanges();
      tick();
      fixture.detectChanges();
    }));

    it('sets aria-current="page" on the active portal link', fakeAsync(() => {
      // Act
      TestBed.inject(Router).navigate(['/main/links']);
      tick();
      fixture.detectChanges();

      // Assert
      const active = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLElement>('[aria-current="page"]');
      expect(active).withContext('one link should have aria-current="page"').toBeTruthy();
      expect(active?.textContent?.trim()).toBe('Main Portal');
    }));

    it('removes aria-current from the previous link when navigating to another portal', fakeAsync(() => {
      const router = TestBed.inject(Router);

      // Navigate to main first
      router.navigate(['/main/links']);
      tick();
      fixture.detectChanges();

      // Navigate to staging
      router.navigate(['/staging/links']);
      tick();
      fixture.detectChanges();

      // Only one link should be active
      const activeLinks = (fixture.nativeElement as HTMLElement)
        .querySelectorAll('[aria-current="page"]');
      expect(activeLinks.length).toBe(1);
      expect(activeLinks[0]?.textContent?.trim()).toBe('Staging');
    }));

    it('does not set aria-current on Help link when a portal is active', fakeAsync(() => {
      TestBed.inject(Router).navigate(['/main/links']);
      tick();
      fixture.detectChanges();

      // Identify Help link by its href (rendered by routerLink)
      const helpLink = (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLElement>('a.toolbar__link[href="/help"]');

      expect(helpLink).withContext('Help link should be in the DOM').toBeTruthy();
      expect(helpLink?.getAttribute('aria-current')).toBeNull();
    }));
  });
});
