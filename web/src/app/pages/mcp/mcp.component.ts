import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipsModule } from '@angular/material/chips';
import { MatDividerModule } from '@angular/material/divider';
import { CONNECT_BASE_URL } from '../../shared/tokens/connect-base-url.token';

interface McpTool {
  name: string;
  description: string;
  filters: string[];
}

@Component({
  selector: 'app-mcp',
  standalone: true,
  imports: [
    MatCardModule,
    MatTableModule,
    MatButtonModule,
    MatIconModule,
    MatChipsModule,
    MatDividerModule,
  ],
  templateUrl: './mcp.component.html',
  styleUrl: './mcp.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class McpComponent {
  private readonly baseUrl = inject(CONNECT_BASE_URL);
  private readonly destroyRef = inject(DestroyRef);

  readonly mcpEndpoint = `${this.baseUrl}/mcp`;

  readonly tools: readonly McpTool[] = [
    {
      name: 'search_fqdns',
      description: 'Search for FQDNs (Fully Qualified Domain Names) in the SRE Portal. Returns a list of DNS entries matching the search criteria.',
      filters: ['query', 'source', 'group', 'portal', 'namespace'],
    },
    {
      name: 'list_portals',
      description: 'List all available portals in the SRE Portal. Portals are entry points that group DNS entries together.',
      filters: [],
    },
    {
      name: 'get_fqdn_details',
      description: 'Get detailed information about a specific FQDN. Returns the full DNS record details including targets, record type, and metadata.',
      filters: ['fqdn'],
    },
  ];

  readonly displayedColumns = ['name', 'description'];

  // Plain strings — derived once from mcpEndpoint, no need for signal
  readonly claudeDesktopConfig = JSON.stringify({
    mcpServers: { sreportal: { url: this.mcpEndpoint } },
  }, null, 2);

  readonly claudeCodeCommand = `claude mcp add sreportal --transport http ${this.mcpEndpoint}`;

  private readonly _copiedStates = signal<Record<string, boolean>>({});
  readonly copiedStates = this._copiedStates.asReadonly();

  copyToClipboard(text: string, key: string): void {
    navigator.clipboard.writeText(text).then(() => {
      this._copiedStates.update(s => ({ ...s, [key]: true }));
      const id = setTimeout(() => {
        this._copiedStates.update(s => ({ ...s, [key]: false }));
      }, 2000);
      this.destroyRef.onDestroy(() => clearTimeout(id));
    });
  }
}
