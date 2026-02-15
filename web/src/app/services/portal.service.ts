import { Injectable } from '@angular/core';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import { PortalService, ListPortalsRequestSchema } from '../../gen/sreportal/v1/portal_pb';
import type { Portal } from '../../gen/sreportal/v1/portal_pb';

@Injectable({
  providedIn: 'root'
})
export class PortalServiceClient {
  private transport = createConnectTransport({
    baseUrl: window.location.origin,
  });

  private client = createClient(PortalService, this.transport);

  async listPortals(namespace?: string): Promise<Portal[]> {
    const request = create(ListPortalsRequestSchema, {
      namespace: namespace || '',
    });

    const response = await this.client.listPortals(request);
    return response.portals;
  }
}
