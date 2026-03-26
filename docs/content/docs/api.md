# API Reference

## Packages
- [sreportal.io/v1alpha1](#sreportaliov1alpha1)


## sreportal.io/v1alpha1

### Resource Types
- [sreportal.io/v1alpha1.Alertmanager](#sreportaliov1alpha1alertmanager)
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)
- [sreportal.io/v1alpha1.FlowEdgeSet](#sreportaliov1alpha1flowedgeset)
- [sreportal.io/v1alpha1.FlowNodeSet](#sreportaliov1alpha1flownodeset)
- [sreportal.io/v1alpha1.NetworkFlowDiscovery](#sreportaliov1alpha1networkflowdiscovery)
- [sreportal.io/v1alpha1.Portal](#sreportaliov1alpha1portal)
- [sreportal.io/v1alpha1.Release](#sreportaliov1alpha1release)


#### sreportal.io/v1alpha1.Alertmanager

Alertmanager is the Schema for the alertmanagers API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Alertmanager` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.AlertmanagerSpec](#sreportaliov1alpha1alertmanagerspec)_ | spec defines the desired state of Alertmanager |   |   |
| `status` _[sreportal.io/v1alpha1.AlertmanagerStatus](#sreportaliov1alpha1alertmanagerstatus)_ | status defines the observed state of Alertmanager |   |   |



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



#### sreportal.io/v1alpha1.FlowEdgeSet

FlowEdgeSet stores the discovered flow edges for a NetworkFlowDiscovery resource.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `FlowEdgeSet` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.FlowEdgeSetSpec](#sreportaliov1alpha1flowedgesetspec)_ |   |   |   |
| `status` _[sreportal.io/v1alpha1.FlowEdgeSetStatus](#sreportaliov1alpha1flowedgesetstatus)_ |   |   |   |



#### sreportal.io/v1alpha1.FlowNodeSet

FlowNodeSet stores the discovered flow nodes for a NetworkFlowDiscovery resource.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `FlowNodeSet` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.FlowNodeSetSpec](#sreportaliov1alpha1flownodesetspec)_ |   |   |   |
| `status` _[sreportal.io/v1alpha1.FlowNodeSetStatus](#sreportaliov1alpha1flownodesetstatus)_ |   |   |   |



#### sreportal.io/v1alpha1.NetworkFlowDiscovery

NetworkFlowDiscovery is the Schema for the networkflowdiscoveries API. It discovers network flows from Kubernetes NetworkPolicies and FQDNNetworkPolicies.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `NetworkFlowDiscovery` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.NetworkFlowDiscoverySpec](#sreportaliov1alpha1networkflowdiscoveryspec)_ | spec defines the desired state of NetworkFlowDiscovery |   |   |
| `status` _[sreportal.io/v1alpha1.NetworkFlowDiscoveryStatus](#sreportaliov1alpha1networkflowdiscoverystatus)_ | status defines the observed state of NetworkFlowDiscovery |   |   |



#### sreportal.io/v1alpha1.Portal

Portal is the Schema for the portals API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Portal` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.PortalSpec](#sreportaliov1alpha1portalspec)_ | spec defines the desired state of Portal |   |   |
| `status` _[sreportal.io/v1alpha1.PortalStatus](#sreportaliov1alpha1portalstatus)_ | status defines the observed state of Portal |   |   |



#### sreportal.io/v1alpha1.Release

Release is the Schema for the releases API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Release` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.ReleaseSpec](#sreportaliov1alpha1releasespec)_ | spec defines the desired state of Release |   |   |
| `status` _[sreportal.io/v1alpha1.ReleaseStatus](#sreportaliov1alpha1releasestatus)_ | status defines the observed state of Release |   |   |



#### sreportal.io/v1alpha1.AlertmanagerSpec

AlertmanagerSpec defines the desired state of Alertmanager

_Appears in:_
- [sreportal.io/v1alpha1.Alertmanager](#sreportaliov1alpha1alertmanager)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this Alertmanager resource is linked to |   |   |
| `url` _[sreportal.io/v1alpha1.AlertmanagerURL](#sreportaliov1alpha1alertmanagerurl)_ | url contains the Alertmanager API endpoints |   |   |
| `isRemote` _boolean_ | IsRemote indicates that the corresponding portal is remote and the operator should fetch alerts from the remote portal instead of local Alertmanager API. This field is used to determine how to fetch alerts. |   |   |



#### sreportal.io/v1alpha1.AlertmanagerURL

AlertmanagerURL holds the local and remote Alertmanager API URLs

_Appears in:_
- [sreportal.io/v1alpha1.AlertmanagerSpec](#sreportaliov1alpha1alertmanagerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `local` _string_ | local is the URL used by the controller to fetch active alerts (e.g. http://alertmanager.monitoring:9093) |   | Pattern: `^https?://.*` |
| `remote` _string_ | remote is an optional externally-reachable URL for dashboard links |   | Pattern: `^https?://.*` |



#### sreportal.io/v1alpha1.AlertmanagerStatus

AlertmanagerStatus defines the observed state of Alertmanager.
remote is an optional externally-reachable URL for dashboard links

_Appears in:_
- [sreportal.io/v1alpha1.Alertmanager](#sreportaliov1alpha1alertmanager)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `activeAlerts` _[sreportal.io/v1alpha1.AlertStatus](#sreportaliov1alpha1alertstatus) array_ | activeAlerts is the list of currently firing alerts retrieved from the Alertmanager API |   |   |
| `silences` _[sreportal.io/v1alpha1.SilenceStatus](#sreportaliov1alpha1silencestatus) array_ | silences is the list of active silences for identifying silenced alerts |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Alertmanager resource. |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |



#### sreportal.io/v1alpha1.AlertStatus

AlertStatus represents a single active alert from Alertmanager

_Appears in:_
- [sreportal.io/v1alpha1.AlertmanagerStatus](#sreportaliov1alpha1alertmanagerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fingerprint` _string_ | fingerprint is the unique identifier of the alert |   |   |
| `labels` _[sreportal.io/v1alpha1.map[string]string](#sreportaliov1alpha1map[string]string)_ | labels are the identifying key-value pairs of the alert |   |   |
| `annotations` _[sreportal.io/v1alpha1.map[string]string](#sreportaliov1alpha1map[string]string)_ | annotations are additional informational key-value pairs |   |   |
| `state` _string_ | state is the alert state (active, suppressed, unprocessed) |   | Enum: [active suppressed unprocessed] |
| `startsAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | startsAt is when the alert started firing |   |   |
| `endsAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | endsAt is when the alert is expected to resolve |   |   |
| `updatedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | updatedAt is the last time the alert was updated |   |   |
| `receivers` _string array_ | receivers are the notification integrations this alert is routed to |   |   |
| `silencedBy` _string array_ | silencedBy contains the IDs of silences that suppress this alert |   |   |



#### sreportal.io/v1alpha1.SilenceStatus

SilenceStatus represents a silence from Alertmanager (for identifying silenced alerts)

_Appears in:_
- [sreportal.io/v1alpha1.AlertmanagerStatus](#sreportaliov1alpha1alertmanagerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ |   |   |   |
| `matchers` _[sreportal.io/v1alpha1.MatcherStatus](#sreportaliov1alpha1matcherstatus) array_ |   |   |   |
| `startsAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ |   |   |   |
| `endsAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ |   |   |   |
| `status` _string_ |   |   |   |
| `createdBy` _string_ |   |   |   |
| `comment` _string_ |   |   |   |
| `updatedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ |   |   |   |



#### sreportal.io/v1alpha1.MatcherStatus

MatcherStatus is a label matcher within a silence

_Appears in:_
- [sreportal.io/v1alpha1.SilenceStatus](#sreportaliov1alpha1silencestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |   |   |   |
| `value` _string_ |   |   |   |
| `isRegex` _boolean_ |   |   |   |



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
| `syncStatus` _string_ | syncStatus indicates whether the FQDN is correctly resolved in DNS. sync: the FQDN resolves to the expected type and targets. notavailable: the FQDN does not exist in DNS. notsync: the FQDN exists but resolves to different targets or type. |   | Enum: [sync notavailable notsync ] |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this FQDN was last observed |   |   |
| `originRef` _[sreportal.io/v1alpha1.OriginResourceRef](#sreportaliov1alpha1originresourceref)_ | originRef identifies the Kubernetes resource (Service, Ingress, DNSEndpoint) that produced this FQDN via external-dns. Not set for manual entries. |   |   |



#### sreportal.io/v1alpha1.DNSRecordSpec

DNSRecordSpec defines the desired state of DNSRecord

_Appears in:_
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sourceType` _string_ | sourceType indicates the external-dns source type that provides this record |   | Enum: [service ingress dnsendpoint istio-gateway istio-virtualservice gateway-httproute gateway-grpcroute gateway-tlsroute gateway-tcproute gateway-udproute] |
| `portalRef` _string_ | portalRef is the name of the Portal this record belongs to |   |   |



#### sreportal.io/v1alpha1.DNSRecordStatus

DNSRecordStatus defines the observed state of DNSRecord
portalRef is the name of the Portal this record belongs to

_Appears in:_
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[sreportal.io/v1alpha1.EndpointStatus](#sreportaliov1alpha1endpointstatus) array_ | endpoints contains the DNS endpoints discovered from this source |   |   |
| `endpointsHash` _string_ | endpointsHash is a SHA-256 digest of the source-provided endpoint data (DNSName, RecordType, Targets, Labels). It is used by the SourceReconciler to skip status updates when endpoints have not changed between ticks. |   |   |
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
| `syncStatus` _string_ | syncStatus indicates whether the endpoint is correctly resolved in DNS. sync: the FQDN resolves to the expected type and targets. notavailable: the FQDN does not exist in DNS. notsync: the FQDN exists but resolves to different targets or type. |   | Enum: [sync notavailable notsync ] |
| `lastSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastSeen is the timestamp when this endpoint was last observed |   |   |



#### sreportal.io/v1alpha1.FlowEdgeSetSpec

FlowEdgeSetSpec defines the desired state of FlowEdgeSet.

_Appears in:_
- [sreportal.io/v1alpha1.FlowEdgeSet](#sreportaliov1alpha1flowedgeset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `discoveryRef` _string_ | discoveryRef is the name of the parent NetworkFlowDiscovery resource |   |   |



#### sreportal.io/v1alpha1.FlowEdgeSetStatus

FlowEdgeSetStatus defines the observed state of FlowEdgeSet.
discoveryRef is the name of the parent NetworkFlowDiscovery resource

_Appears in:_
- [sreportal.io/v1alpha1.FlowEdgeSet](#sreportaliov1alpha1flowedgeset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `edges` _[sreportal.io/v1alpha1.FlowEdge](#sreportaliov1alpha1flowedge) array_ | edges are the directional flow relations between nodes |   |   |



#### sreportal.io/v1alpha1.FlowNodeSetSpec

FlowNodeSetSpec defines the desired state of FlowNodeSet.

_Appears in:_
- [sreportal.io/v1alpha1.FlowNodeSet](#sreportaliov1alpha1flownodeset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `discoveryRef` _string_ | discoveryRef is the name of the parent NetworkFlowDiscovery resource |   |   |



#### sreportal.io/v1alpha1.FlowNodeSetStatus

FlowNodeSetStatus defines the observed state of FlowNodeSet.
discoveryRef is the name of the parent NetworkFlowDiscovery resource

_Appears in:_
- [sreportal.io/v1alpha1.FlowNodeSet](#sreportaliov1alpha1flownodeset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodes` _[sreportal.io/v1alpha1.FlowNode](#sreportaliov1alpha1flownode) array_ | nodes are all discovered services, databases, crons, and external endpoints |   |   |



#### sreportal.io/v1alpha1.NetworkFlowDiscoverySpec

NetworkFlowDiscoverySpec defines the desired state of NetworkFlowDiscovery.

_Appears in:_
- [sreportal.io/v1alpha1.NetworkFlowDiscovery](#sreportaliov1alpha1networkflowdiscovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this resource is linked to |   |   |
| `namespaces` _string array_ | namespaces is an optional list of namespaces to scan. When empty, all namespaces are scanned. |   |   |
| `isRemote` _boolean_ | isRemote indicates that the corresponding portal is remote and the operator should fetch network flows from the remote portal Connect API instead of scanning local Kubernetes NetworkPolicies. |   |   |
| `remoteURL` _string_ | remoteURL is the base URL of the remote SRE Portal to fetch network flows from. Only used when isRemote is true. |   |   |



#### sreportal.io/v1alpha1.NetworkFlowDiscoveryStatus

NetworkFlowDiscoveryStatus defines the observed state of NetworkFlowDiscovery.

_Appears in:_
- [sreportal.io/v1alpha1.NetworkFlowDiscovery](#sreportaliov1alpha1networkflowdiscovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeCount` _integer_ | nodeCount is the number of discovered nodes |   |   |
| `edgeCount` _integer_ | edgeCount is the number of discovered edges |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the resource. |   |   |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastReconcileTime is the timestamp of the last reconciliation |   |   |



#### sreportal.io/v1alpha1.FlowNode

FlowNode represents a service, database, cron job, or external endpoint.

_Appears in:_
- [sreportal.io/v1alpha1.FlowNodeSetStatus](#sreportaliov1alpha1flownodesetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ | id is the unique node identifier (e.g. "service:core:my-account-api") |   |   |
| `label` _string_ | label is the human-readable name |   |   |
| `namespace` _string_ | namespace is the Kubernetes namespace |   |   |
| `nodeType` _string_ | nodeType is one of: service, cron, database, messaging, external |   |   |
| `group` _string_ | group is the logical group (namespace name by default) |   |   |



#### sreportal.io/v1alpha1.FlowEdge

FlowEdge represents a directional flow between two nodes.

_Appears in:_
- [sreportal.io/v1alpha1.FlowEdgeSetStatus](#sreportaliov1alpha1flowedgesetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `from` _string_ | from is the source node id |   |   |
| `to` _string_ | to is the target node id |   |   |
| `edgeType` _string_ | edgeType describes the flow type (e.g. internal, cross-ns, cron, database, messaging, external) |   |   |



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



#### sreportal.io/v1alpha1.ReleaseSpec

ReleaseSpec defines the desired state of Release

_Appears in:_
- [sreportal.io/v1alpha1.Release](#sreportaliov1alpha1release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entries` _[sreportal.io/v1alpha1.ReleaseEntry](#sreportaliov1alpha1releaseentry) array_ | entries is the list of release events for this day |   |   |



#### sreportal.io/v1alpha1.ReleaseEntry

ReleaseEntry represents a single release event

_Appears in:_
- [sreportal.io/v1alpha1.ReleaseSpec](#sreportaliov1alpha1releasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | type is the kind of release (e.g., "deployment", "rollback", "hotfix") |   |   |
| `version` _string_ | version is the version string of the release |   |   |
| `origin` _string_ | origin identifies where the release came from (e.g., "ci/cd", "manual", service name) |   |   |
| `date` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | date is the timestamp of the release |   |   |
| `author` _string_ | author is the author of the release |   |   |
| `message` _string_ | message is the message of the release |   |   |
| `link` _string_ | link is the link to the release |   |   |



#### sreportal.io/v1alpha1.ReleaseStatus

ReleaseStatus defines the observed state of Release.
link is the link to the release

_Appears in:_
- [sreportal.io/v1alpha1.Release](#sreportaliov1alpha1release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entryCount` _integer_ | entryCount is the number of release entries in this CR |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Release resource. |   |   |





