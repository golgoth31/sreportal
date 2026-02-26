import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, computed, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { catchError, map, of } from 'rxjs';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { DnsFacade } from '../../application/dns.facade';
import { PortalServiceClient } from '../../services/portal.service';
import { FqdnGroupComponent } from './components/fqdn-group/fqdn-group.component';

@Component({
  selector: 'app-links',
  standalone: true,
  imports: [
    FqdnGroupComponent,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatButtonModule,
    MatProgressSpinnerModule,
    MatIconModule,
    MatTooltipModule,
  ],
  templateUrl: './links.component.html',
  styleUrl: './links.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [DnsFacade],
})
export class LinksComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);
  private readonly facade = inject(DnsFacade);
  private readonly portalServiceClient = inject(PortalServiceClient);

  // ── DNS state (from facade) ─────────────────────────────────────────────────
  readonly loading = this.facade.loading;
  readonly error = this.facade.error;
  readonly searchTerm = this.facade.searchTerm;
  readonly groupFilter = this.facade.groupFilter;
  readonly groups = this.facade.groups;
  readonly filteredCount = this.facade.filteredCount;
  readonly totalCount = this.facade.totalCount;
  readonly groupedByGroup = this.facade.groupedByGroup;
  readonly hasActiveFilters = computed(
    () => !!this.facade.searchTerm() || !!this.facade.groupFilter(),
  );

  // ── Remote portal detection ─────────────────────────────────────────────────
  private readonly portals = toSignal(
    this.portalServiceClient.listPortals().pipe(catchError(() => of([]))),
    { initialValue: [] },
  );

  private readonly _portalName = toSignal(
    this.route.params.pipe(map(p => String(p['portalName'] ?? 'main'))),
    { initialValue: 'main' },
  );

  /** Non-null only when viewing a remote portal — drives the external link button. */
  readonly remotePortalUrl = computed(() => {
    const name = this._portalName();
    const portal = this.portals().find(p => (p.subPath || p.name) === name);
    return (portal?.isRemote === true && portal.url) ? portal.url : null;
  });

  ngOnInit(): void {
    this.route.params
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(params => {
        this.facade.loadFqdns(params['portalName'] ?? 'main');
      });
  }

  onSearch(event: Event): void {
    this.facade.setSearchTerm((event.target as HTMLInputElement).value);
  }

  onGroupFilter(value: string): void {
    this.facade.setGroupFilter(value);
  }

  clearFilters(): void {
    this.facade.clearFilters();
  }
}
