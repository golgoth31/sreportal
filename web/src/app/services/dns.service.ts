import { Injectable } from '@angular/core';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import { DNSService, ListFQDNsRequestSchema, StreamFQDNsRequestSchema } from '../../gen/sreportal/v1/dns_pb';
import type { FQDN, StreamFQDNsResponse } from '../../gen/sreportal/v1/dns_pb';

@Injectable({
  providedIn: 'root'
})
export class DnsService {
  private transport = createConnectTransport({
    baseUrl: window.location.origin,
  });

  private client = createClient(DNSService, this.transport);

  async listFQDNs(namespace?: string, source?: string, search?: string, portal?: string): Promise<FQDN[]> {
    const request = create(ListFQDNsRequestSchema, {
      namespace: namespace || '',
      source: source || '',
      search: search || '',
      portal: portal || '',
    });

    const response = await this.client.listFQDNs(request);
    return response.fqdns;
  }

  async *streamFQDNs(namespace?: string): AsyncGenerator<StreamFQDNsResponse> {
    const request = create(StreamFQDNsRequestSchema, {
      namespace: namespace || '',
    });
    for await (const update of this.client.streamFQDNs(request)) {
      yield update;
    }
  }
}
