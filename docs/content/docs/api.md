# API Reference

## Packages
- [sreportal.io/v1alpha1](#sreportaliov1alpha1)


## sreportal.io/v1alpha1

### Resource Types
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)
- [sreportal.io/v1alpha1.Portal](#sreportaliov1alpha1portal)


#### sreportal.io/v1alpha1.DNS

DNS is the Schema for the dns API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `DNS` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.DNSSpec](#sreportaliov1alpha1dnsspec)_ | spec defines the desired state of DNS |   |   |
| `status` _[sreportal.io/v1alpha1.DNSStatus](#sreportaliov1alpha1dnsstatus)_ | status defines the observed state of DNS |   |   |



#### sreportal.io/v1alpha1.DNSRecord

DNSRecord is the Schema for the dnsrecords API. It represents DNS endpoints discovered from a specific external-dns source.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `DNSRecord` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.DNSRecordSpec](#sreportaliov1alpha1dnsrecordspec)_ | spec defines the desired state of DNSRecord |   |   |
| `status` _[sreportal.io/v1alpha1.DNSRecordStatus](#sreportaliov1alpha1dnsrecordstatus)_ | status defines the observed state of DNSRecord |   |   |



#### sreportal.io/v1alpha1.Portal

Portal is the Schema for the portals API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Portal` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.PortalSpec](#sreportaliov1alpha1portalspec)_ | spec defines the desired state of Portal |   |   |
| `status` _[sreportal.io/v1alpha1.PortalStatus](#sreportaliov1alpha1portalstatus)_ | status defines the observed state of Portal |   |   |



#### sreportal.io/v1alpha1.DNSSpec

DNSSpec defines the desired state of DNS

_Appears in:_
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this DNS resource is linked to |   |   |
| `groups` _[sreportal.io/v1alpha1.DNSGroup](#sreportaliov1alpha1dnsgroup) array_ | groups is a list of DNS entry groups for organizing entries in the UI |   |   |



#### sreportal.io/v1alpha1.DNSGroup

DNSGroup represents a group of DNS entries

_Appears in:_
- [sreportal.io/v1alpha1.DNSSpec](#sreportaliov1alpha1dnsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the display name for this group |   |   |
| `description` _string_ | description is an optional description for the group |   |   |
| `entries` _[sreportal.io/v1alpha1.DNSEntry](#sreportaliov1alpha1dnsentry) array_ | entries is a list of DNS entries in this group |   |   |



#### sreportal.io/v1alpha1.DNSEntry

DNSEntry represents a manual DNS entry

_Appears in:_
- [sreportal.io/v1alpha1.DNSGroup](#sreportaliov1alpha1dnsgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fqdn` _string_ | fqdn is the fully qualified domain name |   |   |
| `description` _string_ | description is an optional description for the DNS entry |   |   |



#### sreportal.io/v1alpha1.DNSStatus

DNSStatus defines the observed state of DNS.

_Appears in:_
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `groups` _[sreportal.io/v1alpha1.FQDNGroupStatus](#sreportaliov1alpha1fqdngroupstatus) array_ | groups is the list of FQDN groups with their status |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the DNS resource. |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |



#### sreportal.io/v1alpha1.FQDNGroupStatus

FQDNGroupStatus represents a group of FQDNs in the status

_Appears in:_
- [sreportal.io/v1alpha1.DNSStatus](#sreportaliov1alpha1dnsstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the group name |   |   |
| `description` _string_ | description is the group description |   |   |
| `source` _string_ | source indicates where this group came from (manual, external-dns, or remote) |   | Enum: [manual external-dns remote] |
| `fqdns` _[sreportal.io/v1alpha1.FQDNStatus](#sreportaliov1alpha1fqdnstatus) array_ | fqdns is the list of FQDNs in this group |   |   |



#### sreportal.io/v1alpha1.OriginResourceRef

OriginResourceRef identifies the Kubernetes resource that produced an FQDN. Only populated for FQDNs discovered via external-dns sources.

_Appears in:_
- [sreportal.io/v1alpha1.FQDNStatus](#sreportaliov1alpha1fqdnstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | kind is the Kubernetes resource kind (e.g. Service, Ingress, DNSEndpoint) |   |   |
| `namespace` _string_ | namespace is the Kubernetes namespace of the origin resource |   |   |
| `name` _string_ | name is the name of the origin Kubernetes resource |   |   |



#### sreportal.io/v1alpha1.FQDNStatus

FQDNStatus represents the status of an aggregated FQDN

_Appears in:_
- [sreportal.io/v1alpha1.FQDNGroupStatus](#sreportaliov1alpha1fqdngroupstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fqdn` _string_ | fqdn is the fully qualified domain name |   |   |
| `description` _string_ | description is an optional description for the FQDN |   |   |
| `recordType` _string_ | recordType is the DNS record type (A, AAAA, CNAME, etc.) |   |   |
| `targets` _string array_ | targets is the list of target addresses for this FQDN |   |   |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this FQDN was last observed |   |   |
| `originRef` _[sreportal.io/v1alpha1.OriginResourceRef](#sreportaliov1alpha1originresourceref)_ | originRef identifies the Kubernetes resource (Service, Ingress, DNSEndpoint) that produced this FQDN via external-dns. Not set for manual entries. |   |   |



#### sreportal.io/v1alpha1.DNSRecordSpec

DNSRecordSpec defines the desired state of DNSRecord

_Appears in:_
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sourceType` _string_ | sourceType indicates the external-dns source type that provides this record |   | Enum: [service ingress dnsendpoint istio-gateway istio-virtualservice] |
| `portalRef` _string_ | portalRef is the name of the Portal this record belongs to |   |   |



#### sreportal.io/v1alpha1.DNSRecordStatus

DNSRecordStatus defines the observed state of DNSRecord
portalRef is the name of the Portal this record belongs to

_Appears in:_
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[sreportal.io/v1alpha1.EndpointStatus](#sreportaliov1alpha1endpointstatus) array_ | endpoints contains the DNS endpoints discovered from this source |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the DNSRecord resource |   |   |



#### sreportal.io/v1alpha1.EndpointStatus

EndpointStatus represents a single DNS endpoint discovered from external-dns

_Appears in:_
- [sreportal.io/v1alpha1.DNSRecordStatus](#sreportaliov1alpha1dnsrecordstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dnsName` _string_ | dnsName is the fully qualified domain name |   |   |
| `recordType` _string_ | recordType is the DNS record type (A, AAAA, CNAME, TXT, etc.) |   |   |
| `targets` _string array_ | targets is the list of target addresses for this endpoint |   |   |
| `ttl` _integer_ | ttl is the DNS record TTL in seconds |   |   |
| `labels` _[sreportal.io/v1alpha1.map[string]string](#sreportaliov1alpha1map[string]string)_ | labels contains the endpoint labels from external-dns |   |   |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this endpoint was last observed |   |   |



#### sreportal.io/v1alpha1.PortalSpec

PortalSpec defines the desired state of Portal

_Appears in:_
- [sreportal.io/v1alpha1.Portal](#sreportaliov1alpha1portal)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `title` _string_ | title is the display title for this portal |   |   |
| `main` _boolean_ | main marks this portal as the default portal for unmatched FQDNs |   |   |
| `subPath` _string_ | subPath is the URL subpath for this portal (defaults to metadata.name) |   |   |
| `remote` _[sreportal.io/v1alpha1.RemotePortalSpec](#sreportaliov1alpha1remoteportalspec)_ | remote configures this portal to fetch data from a remote SRE Portal instance. When set, the operator will fetch DNS information from the remote portal instead of collecting data from the local cluster. This field cannot be set when main is true. |   |   |



#### sreportal.io/v1alpha1.RemotePortalSpec

RemotePortalSpec defines the configuration for fetching data from a remote portal.

_Appears in:_
- [sreportal.io/v1alpha1.PortalSpec](#sreportaliov1alpha1portalspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | url is the base URL of the remote SRE Portal instance. |   | Pattern: `^https?://.*` |
| `portal` _string_ | portal is the name of the portal to target on the remote instance. If not set, the main portal of the remote instance will be used. |   |   |
| `tls` _[sreportal.io/v1alpha1.RemoteTLSConfig](#sreportaliov1alpha1remotetlsconfig)_ | tls configures TLS settings for connecting to the remote portal. If not set, the default system TLS configuration is used. |   |   |



#### sreportal.io/v1alpha1.RemoteTLSConfig

RemoteTLSConfig defines the TLS configuration for connecting to a remote portal.

_Appears in:_
- [sreportal.io/v1alpha1.RemotePortalSpec](#sreportaliov1alpha1remoteportalspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `insecureSkipVerify` _boolean_ | insecureSkipVerify disables TLS certificate verification when connecting to the remote portal. Use with caution: this makes the connection susceptible to man-in-the-middle attacks. |   |   |
| `caSecretRef` _[sreportal.io/v1alpha1.SecretRef](#sreportaliov1alpha1secretref)_ | caSecretRef references a Secret containing a custom CA certificate bundle. The Secret must contain the key "ca.crt". |   |   |
| `certSecretRef` _[sreportal.io/v1alpha1.SecretRef](#sreportaliov1alpha1secretref)_ | certSecretRef references a Secret containing a client certificate and key for mTLS. The Secret must contain the keys "tls.crt" and "tls.key". |   |   |



#### sreportal.io/v1alpha1.SecretRef

SecretRef is a reference to a Kubernetes Secret in the same namespace.

_Appears in:_
- [sreportal.io/v1alpha1.RemoteTLSConfig](#sreportaliov1alpha1remotetlsconfig)
- [sreportal.io/v1alpha1.RemoteTLSConfig](#sreportaliov1alpha1remotetlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the name of the Secret. |   |   |



#### sreportal.io/v1alpha1.PortalStatus

PortalStatus defines the observed state of Portal.
name is the name of the Secret.

_Appears in:_
- [sreportal.io/v1alpha1.Portal](#sreportaliov1alpha1portal)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | ready indicates if the portal is fully configured |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Portal resource. |   |   |
| `remoteSync` _[sreportal.io/v1alpha1.RemoteSyncStatus](#sreportaliov1alpha1remotesyncstatus)_ | remoteSync contains the status of synchronization with a remote portal. This is only populated when spec.remote is set. |   |   |



#### sreportal.io/v1alpha1.RemoteSyncStatus

RemoteSyncStatus contains status information about remote portal synchronization.

_Appears in:_
- [sreportal.io/v1alpha1.PortalStatus](#sreportaliov1alpha1portalstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastSyncTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSyncTime is the timestamp of the last successful synchronization. |   |   |
| `lastSyncError` _string_ | lastSyncError contains the error message from the last failed synchronization attempt. Empty if the last sync was successful. |   |   |
| `remoteTitle` _string_ | remoteTitle is the title of the remote portal as fetched from the remote server. |   |   |
| `fqdnCount` _integer_ | fqdnCount is the number of FQDNs fetched from the remote portal. |   |   |





