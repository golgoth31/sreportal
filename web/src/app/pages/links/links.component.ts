import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, inject } from '@angular/core';
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

  onGroupFilter(value: string): void {
    this.facade.setGroupFilter(value);
  }

  clearFilters(): void {
    this.facade.clearFilters();
  }
}
