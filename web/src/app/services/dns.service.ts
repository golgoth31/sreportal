import { Injectable, inject } from '@angular/core';
import { Observable, from, map } from 'rxjs';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import { DNSService, ListFQDNsRequestSchema } from '../../gen/sreportal/v1/dns_pb';
import type { FQDN } from '../../gen/sreportal/v1/dns_pb';
import { CONNECT_BASE_URL } from '../shared/tokens/connect-base-url.token';

@Injectable({
  providedIn: 'root',
})
export class DnsService {
  private readonly client = createClient(
    DNSService,
    createConnectTransport({ baseUrl: inject(CONNECT_BASE_URL) }),
  );

  listFQDNs(portal = ''): Observable<FQDN[]> {
    const request = create(ListFQDNsRequestSchema, { portal });
    return from(this.client.listFQDNs(request)).pipe(map(r => r.fqdns));
  }
}
