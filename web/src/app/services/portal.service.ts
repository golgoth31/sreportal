import { Injectable, inject } from '@angular/core';
import { Observable, from, map } from 'rxjs';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import { PortalService, ListPortalsRequestSchema } from '../../gen/sreportal/v1/portal_pb';
import type { Portal } from '../../gen/sreportal/v1/portal_pb';
import { CONNECT_BASE_URL } from '../shared/tokens/connect-base-url.token';

@Injectable({
  providedIn: 'root',
})
export class PortalServiceClient {
  private readonly client = createClient(
    PortalService,
    createConnectTransport({ baseUrl: inject(CONNECT_BASE_URL) }),
  );

  listPortals(): Observable<Portal[]> {
    return from(this.client.listPortals(create(ListPortalsRequestSchema, {}))).pipe(
      map(r => r.portals),
    );
  }
}
