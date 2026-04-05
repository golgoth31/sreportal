import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  NetworkPolicyService,
  ListNetworkPoliciesRequestSchema,
  type NetpolNode as ProtoNode,
  type NetpolEdge as ProtoEdge,
} from "@/gen/sreportal/v1/netpol_pb";
import type { NetpolNode, NetpolEdge, NetpolGraph } from "../domain/netpol.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(NetworkPolicyService, transport);

function toDomainNode(n: ProtoNode): NetpolNode {
  return {
    id: n.id,
    label: n.label,
    namespace: n.namespace,
    nodeType: n.nodeType as NetpolNode["nodeType"],
    group: n.group,
  };
}

function toDomainEdge(e: ProtoEdge): NetpolEdge {
  return {
    from: e.from,
    to: e.to,
    edgeType: e.edgeType as NetpolEdge["edgeType"],
    lastSeen: e.lastSeen ? new Date(Number(e.lastSeen.seconds) * 1000).toISOString() : null,
  };
}

export interface ListNetpolParams {
  namespace?: string;
  search?: string;
  portal?: string;
}

export async function listNetworkPolicies(
  params: ListNetpolParams = {}
): Promise<NetpolGraph> {
  const request = create(ListNetworkPoliciesRequestSchema, {
    namespace: params.namespace ?? "",
    search: params.search ?? "",
    portal: params.portal ?? "",
  });
  const response = await client.listNetworkPolicies(request);
  return {
    nodes: response.nodes.map(toDomainNode),
    edges: response.edges.map(toDomainEdge),
  };
}
