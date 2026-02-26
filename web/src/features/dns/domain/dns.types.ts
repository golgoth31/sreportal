export interface OriginRef {
  readonly kind: string;
  readonly namespace: string;
  readonly name: string;
}

export interface Fqdn {
  readonly name: string;
  readonly source: string;
  readonly groups: readonly string[];
  readonly description: string;
  readonly recordType: string;
  readonly targets: readonly string[];
  readonly dnsResourceName: string;
  readonly dnsResourceNamespace: string;
  readonly originRef?: OriginRef;
}

export interface FqdnGroup {
  readonly name: string;
  readonly source: string;
  readonly fqdns: readonly Fqdn[];
}
