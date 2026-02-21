import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-button',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button
      [class]="classes()"
      [disabled]="disabled() || loading()"
      [attr.type]="type()"
      [attr.aria-busy]="loading() || null"
    >
      @if (loading()) {
        <span class="btn-spinner" aria-hidden="true"></span>
        <span class="sr-only">Loading…</span>
      } @else {
        <ng-content />
      }
    </button>
  `,
  styleUrl: './button.component.scss',
})
export class ButtonComponent {
  variant = input<'default' | 'outline' | 'ghost' | 'destructive'>('default');
  size = input<'default' | 'sm' | 'icon'>('default');
  disabled = input(false);
  loading = input(false);
  type = input<'button' | 'submit' | 'reset'>('button');

  classes = computed(() =>
    ['btn', `btn--${this.variant()}`, `btn--${this.size()}`].join(' ')
  );
}
