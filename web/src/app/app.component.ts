import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { catchError, of } from 'rxjs';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PortalServiceClient } from './services/portal.service';
import { ThemeService } from './services/theme.service';
import type { Portal } from '../gen/sreportal/v1/portal_pb';

@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    MatToolbarModule,
    MatButtonModule,
    MatIconModule,
    MatTooltipModule,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppComponent {
  readonly title = 'SRE Portal';
  readonly theme = inject(ThemeService);

  readonly portals = toSignal(
    inject(PortalServiceClient)
      .listPortals()
      .pipe(catchError(() => of([] as Portal[]))),
    { initialValue: [] as Portal[] },
  );
}
