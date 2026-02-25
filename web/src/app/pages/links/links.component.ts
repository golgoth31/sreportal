import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, computed, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatIconModule } from '@angular/material/icon';
import { DnsFacade } from '../../application/dns.facade';
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

  // Expose signals from facade — template never touches the facade directly
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
