import { Component, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { PortalServiceClient } from './services/portal.service';
import type { Portal } from '../gen/sreportal/v1/portal_pb';

@Component({
  selector: 'app-root',
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent implements OnInit {
  private readonly portalService = inject(PortalServiceClient);

  title = 'SRE Portal';
  portals = signal<Portal[]>([]);

  async ngOnInit(): Promise<void> {
    try {
      const portalList = await this.portalService.listPortals();
      this.portals.set(portalList);
    } catch {
      // Portals will be empty - navigation will still work with default route
    }
  }

  getPortalPath(portal: Portal): string {
    return `/${portal.subPath || portal.name}/links`;
  }
}
