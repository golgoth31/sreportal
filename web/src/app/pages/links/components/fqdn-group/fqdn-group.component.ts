import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { BadgeComponent } from '../../../../shared/ui/badge/badge.component';
import { ButtonComponent } from '../../../../shared/ui/button/button.component';
import type { BadgeVariant } from '../../../../shared/ui/badge/badge.component';
import type { FqdnGroup } from '../../../../application/dns.facade';

@Component({
  selector: 'app-fqdn-group',
  standalone: true,
  imports: [BadgeComponent, ButtonComponent],
  templateUrl: './fqdn-group.component.html',
  styleUrl: './fqdn-group.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FqdnGroupComponent {
  group = input.required<FqdnGroup>();

  private readonly _isOpen = signal(true);
  readonly isOpen = this._isOpen.asReadonly();

  readonly sourceBadgeVariant = computed((): BadgeVariant =>
    this.group().source === 'manual' ? 'success' : 'external'
  );

  readonly sourceIcon = computed(() =>
    this.group().source === 'external-dns' ? 'E' : 'M'
  );

  readonly sourceLabelVariant = computed((): BadgeVariant =>
    this.group().source === 'manual' ? 'success' : 'external'
  );

  toggle(): void {
    this._isOpen.update(open => !open);
  }

  copyToClipboard(text: string): void {
    navigator.clipboard.writeText(text);
  }
}
