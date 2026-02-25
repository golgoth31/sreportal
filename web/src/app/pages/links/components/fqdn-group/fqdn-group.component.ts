import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, input, signal } from '@angular/core';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatTooltipModule } from '@angular/material/tooltip';
import type { FqdnGroup } from '../../../../application/dns.facade';

@Component({
  selector: 'app-fqdn-group',
  standalone: true,
  imports: [
    MatExpansionModule,
    MatChipsModule,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatTooltipModule,
  ],
  templateUrl: './fqdn-group.component.html',
  styleUrl: './fqdn-group.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FqdnGroupComponent {
  group = input.required<FqdnGroup>();

  private readonly destroyRef = inject(DestroyRef);
  private readonly _copiedFqdn = signal<string | null>(null);
  readonly copiedFqdn = this._copiedFqdn.asReadonly();

  readonly sourceLabel = computed(() =>
    this.group().source === 'manual' ? 'Manual' : 'External DNS'
  );

  readonly sourceIcon = computed(() =>
    this.group().source === 'manual' ? 'edit' : 'dns'
  );

  copyToClipboard(text: string): void {
    navigator.clipboard.writeText(text).then(() => {
      this._copiedFqdn.set(text);
      const id = setTimeout(() => this._copiedFqdn.set(null), 2000);
      this.destroyRef.onDestroy(() => clearTimeout(id));
    });
  }
}
