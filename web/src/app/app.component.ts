import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { catchError, of } from 'rxjs';
import { PortalServiceClient } from './services/portal.service';
import type { Portal } from '../gen/sreportal/v1/portal_pb';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppComponent {
  readonly title = 'SRE Portal';

  readonly portals = toSignal(
    inject(PortalServiceClient)
      .listPortals()
      .pipe(catchError(() => of([] as Portal[]))),
    { initialValue: [] as Portal[] },
  );
}
