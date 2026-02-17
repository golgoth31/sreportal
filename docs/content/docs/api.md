# API Reference

## Packages
- [sreportal.my.domain/v1alpha1](#sreportalmydomainv1alpha1)


## sreportal.my.domain/v1alpha1

### Resource Types
- [sreportal.my.domain/v1alpha1.DNS](#sreportalmydomainv1alpha1dns)
- [sreportal.my.domain/v1alpha1.DNSRecord](#sreportalmydomainv1alpha1dnsrecord)
- [sreportal.my.domain/v1alpha1.Portal](#sreportalmydomainv1alpha1portal)


#### sreportal.my.domain/v1alpha1.DNS

DNS is the Schema for the dns API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.my.domain/v1alpha1` |   |   |
| `kind` _string_ | `DNS` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.my.domain/v1alpha1.DNSSpec](#sreportalmydomainv1alpha1dnsspec)_ | spec defines the desired state of DNS |   |   |
| `status` _[sreportal.my.domain/v1alpha1.DNSStatus](#sreportalmydomainv1alpha1dnsstatus)_ | status defines the observed state of DNS |   |   |



#### sreportal.my.domain/v1alpha1.DNSRecord

DNSRecord is the Schema for the dnsrecords API. It represents DNS endpoints discovered from a specific external-dns source.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.my.domain/v1alpha1` |   |   |
| `kind` _string_ | `DNSRecord` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.my.domain/v1alpha1.DNSRecordSpec](#sreportalmydomainv1alpha1dnsrecordspec)_ | spec defines the desired state of DNSRecord |   |   |
| `status` _[sreportal.my.domain/v1alpha1.DNSRecordStatus](#sreportalmydomainv1alpha1dnsrecordstatus)_ | status defines the observed state of DNSRecord |   |   |



#### sreportal.my.domain/v1alpha1.Portal

Portal is the Schema for the portals API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.my.domain/v1alpha1` |   |   |
| `kind` _string_ | `Portal` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.my.domain/v1alpha1.PortalSpec](#sreportalmydomainv1alpha1portalspec)_ | spec defines the desired state of Portal |   |   |
| `status` _[sreportal.my.domain/v1alpha1.PortalStatus](#sreportalmydomainv1alpha1portalstatus)_ | status defines the observed state of Portal |   |   |



#### sreportal.my.domain/v1alpha1.DNSSpec

DNSSpec defines the desired state of DNS

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNS](#sreportalmydomainv1alpha1dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this DNS resource is linked to |   |   |
| `groups` _[sreportal.my.domain/v1alpha1.DNSGroup](#sreportalmydomainv1alpha1dnsgroup) array_ | groups is a list of DNS entry groups for organizing entries in the UI |   |   |



#### sreportal.my.domain/v1alpha1.DNSGroup

DNSGroup represents a group of DNS entries

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSSpec](#sreportalmydomainv1alpha1dnsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the display name for this group |   |   |
| `description` _string_ | description is an optional description for the group |   |   |
| `entries` _[sreportal.my.domain/v1alpha1.DNSEntry](#sreportalmydomainv1alpha1dnsentry) array_ | entries is a list of DNS entries in this group |   |   |



#### sreportal.my.domain/v1alpha1.DNSEntry

DNSEntry represents a manual DNS entry

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSGroup](#sreportalmydomainv1alpha1dnsgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fqdn` _string_ | fqdn is the fully qualified domain name |   |   |
| `description` _string_ | description is an optional description for the DNS entry |   |   |



#### sreportal.my.domain/v1alpha1.DNSStatus

DNSStatus defines the observed state of DNS.

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNS](#sreportalmydomainv1alpha1dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `groups` _[sreportal.my.domain/v1alpha1.FQDNGroupStatus](#sreportalmydomainv1alpha1fqdngroupstatus) array_ | groups is the list of FQDN groups with their status |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the DNS resource. |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |



#### sreportal.my.domain/v1alpha1.FQDNGroupStatus

FQDNGroupStatus represents a group of FQDNs in the status

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSStatus](#sreportalmydomainv1alpha1dnsstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the group name |   |   |
| `description` _string_ | description is the group description |   |   |
| `source` _string_ | source indicates where this group came from (manual, external-dns, or remote) |   | Enum: [manual external-dns remote] |
| `fqdns` _[sreportal.my.domain/v1alpha1.FQDNStatus](#sreportalmydomainv1alpha1fqdnstatus) array_ | fqdns is the list of FQDNs in this group |   |   |



#### sreportal.my.domain/v1alpha1.FQDNStatus

FQDNStatus represents the status of an aggregated FQDN

_Appears in:_
- [sreportal.my.domain/v1alpha1.FQDNGroupStatus](#sreportalmydomainv1alpha1fqdngroupstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fqdn` _string_ | fqdn is the fully qualified domain name |   |   |
| `description` _string_ | description is an optional description for the FQDN |   |   |
| `recordType` _string_ | recordType is the DNS record type (A, AAAA, CNAME, etc.) |   |   |
| `targets` _string array_ | targets is the list of target addresses for this FQDN |   |   |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this FQDN was last observed |   |   |



#### sreportal.my.domain/v1alpha1.DNSRecordSpec

DNSRecordSpec defines the desired state of DNSRecord

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSRecord](#sreportalmydomainv1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sourceType` _string_ | sourceType indicates the external-dns source type that provides this record |   | Enum: [service ingress dnsendpoint istio-gateway istio-virtualservice] |
| `portalRef` _string_ | portalRef is the name of the Portal this record belongs to |   |   |



#### sreportal.my.domain/v1alpha1.DNSRecordStatus

DNSRecordStatus defines the observed state of DNSRecord
portalRef is the name of the Portal this record belongs to

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSRecord](#sreportalmydomainv1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[sreportal.my.domain/v1alpha1.EndpointStatus](#sreportalmydomainv1alpha1endpointstatus) array_ | endpoints contains the DNS endpoints discovered from this source |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the DNSRecord resource |   |   |



#### sreportal.my.domain/v1alpha1.EndpointStatus

EndpointStatus represents a single DNS endpoint discovered from external-dns

_Appears in:_
- [sreportal.my.domain/v1alpha1.DNSRecordStatus](#sreportalmydomainv1alpha1dnsrecordstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dnsName` _string_ | dnsName is the fully qualified domain name |   |   |
| `recordType` _string_ | recordType is the DNS record type (A, AAAA, CNAME, TXT, etc.) |   |   |
| `targets` _string array_ | targets is the list of target addresses for this endpoint |   |   |
| `ttl` _integer_ | ttl is the DNS record TTL in seconds |   |   |
| `labels` _[sreportal.my.domain/v1alpha1.map[string]string](#sreportalmydomainv1alpha1map[string]string)_ | labels contains the endpoint labels from external-dns |   |   |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this endpoint was last observed |   |   |



#### sreportal.my.domain/v1alpha1.PortalSpec

PortalSpec defines the desired state of Portal

_Appears in:_
- [sreportal.my.domain/v1alpha1.Portal](#sreportalmydomainv1alpha1portal)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `title` _string_ | title is the display title for this portal |   |   |
| `main` _boolean_ | main marks this portal as the default portal for unmatched FQDNs |   |   |
| `subPath` _string_ | subPath is the URL subpath for this portal (defaults to metadata.name) |   |   |
| `remote` _[sreportal.my.domain/v1alpha1.RemotePortalSpec](#sreportalmydomainv1alpha1remoteportalspec)_ | remote configures this portal to fetch data from a remote SRE Portal instance. When set, the operator will fetch DNS information from the remote portal instead of collecting data from the local cluster. This field cannot be set when main is true. |   |   |



#### sreportal.my.domain/v1alpha1.RemotePortalSpec

RemotePortalSpec defines the configuration for fetching data from a remote portal.

_Appears in:_
- [sreportal.my.domain/v1alpha1.PortalSpec](#sreportalmydomainv1alpha1portalspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | url is the base URL of the remote SRE Portal instance. |   | Pattern: `^https?://.*` |
| `portal` _string_ | portal is the name of the portal to target on the remote instance. If not set, the main portal of the remote instance will be used. |   |   |



#### sreportal.my.domain/v1alpha1.PortalStatus

PortalStatus defines the observed state of Portal.

_Appears in:_
- [sreportal.my.domain/v1alpha1.Portal](#sreportalmydomainv1alpha1portal)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | ready indicates if the portal is fully configured |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Portal resource. |   |   |
| `remoteSync` _[sreportal.my.domain/v1alpha1.RemoteSyncStatus](#sreportalmydomainv1alpha1remotesyncstatus)_ | remoteSync contains the status of synchronization with a remote portal. This is only populated when spec.remote is set. |   |   |



#### sreportal.my.domain/v1alpha1.RemoteSyncStatus

RemoteSyncStatus contains status information about remote portal synchronization.

_Appears in:_
- [sreportal.my.domain/v1alpha1.PortalStatus](#sreportalmydomainv1alpha1portalstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastSyncTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSyncTime is the timestamp of the last successful synchronization. |   |   |
| `lastSyncError` _string_ | lastSyncError contains the error message from the last failed synchronization attempt. Empty if the last sync was successful. |   |   |
| `remoteTitle` _string_ | remoteTitle is the title of the remote portal as fetched from the remote server. |   |   |
| `fqdnCount` _integer_ | fqdnCount is the number of FQDNs fetched from the remote portal. |   |   |





