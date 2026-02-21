import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DnsFacade } from '../../application/dns.facade';
import { FqdnGroupComponent } from './components/fqdn-group/fqdn-group.component';
import { ButtonComponent } from '../../shared/ui/button/button.component';

@Component({
  selector: 'app-links',
  standalone: true,
  imports: [FqdnGroupComponent, ButtonComponent],
  templateUrl: './links.component.html',
  styleUrl: './links.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [DnsFacade],
})
export class LinksComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);
  readonly facade = inject(DnsFacade);

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

  onGroupFilter(event: Event): void {
    this.facade.setGroupFilter((event.target as HTMLSelectElement).value);
  }

  clearFilters(): void {
    this.facade.clearFilters();
  }
}
