import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { DnsService } from '../../services/dns.service';
import { DnsState } from '../../state/dns.state';

@Component({
  selector: 'app-links',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './links.component.html',
  styleUrl: './links.component.scss'
})
export class LinksComponent implements OnInit {
  private readonly dnsService = inject(DnsService);
  private readonly route = inject(ActivatedRoute);
  readonly state = inject(DnsState);

  searchInput = '';
  portalName = '';

  async ngOnInit(): Promise<void> {
    this.route.params.subscribe(async params => {
      this.portalName = params['portalName'] || 'main';
      await this.loadFqdns();
    });
  }

  async loadFqdns(): Promise<void> {
    this.state.setLoading(true);
    try {
      const fqdns = await this.dnsService.listFQDNs(undefined, undefined, undefined, this.portalName);
      this.state.setFqdns(fqdns);
    } catch (error) {
      this.state.setError(error instanceof Error ? error.message : 'Failed to load FQDNs');
    } finally {
      this.state.setLoading(false);
    }
  }

  onSearch(): void {
    this.state.setSearchTerm(this.searchInput);
  }

  onGroupFilter(group: string): void {
    this.state.setGroupFilter(group);
  }

  clearFilters(): void {
    this.searchInput = '';
    this.state.clearFilters();
  }

  getSourceIcon(source: string): string {
    switch (source) {
      case 'manual':
        return 'M';
      case 'external-dns':
        return 'E';
      default:
        return '?';
    }
  }

  getSourceClass(source: string): string {
    switch (source) {
      case 'manual':
        return 'source-manual';
      case 'external-dns':
        return 'source-external';
      default:
        return 'source-unknown';
    }
  }

  formatDate(timestamp: any): string {
    if (!timestamp) return 'N/A';
    const date = timestamp.toDate ? timestamp.toDate() : new Date(timestamp);
    return date.toLocaleString();
  }

  copyToClipboard(text: string): void {
    navigator.clipboard.writeText(text);
  }
}
