# API Reference

## Packages
- [sreportal.io/v1alpha1](#sreportaliov1alpha1)


## sreportal.io/v1alpha1

### Resource Types
- [sreportal.io/v1alpha1.Alertmanager](#sreportaliov1alpha1alertmanager)
- [sreportal.io/v1alpha1.Component](#sreportaliov1alpha1component)
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)
- [sreportal.io/v1alpha1.DNSRecord](#sreportaliov1alpha1dnsrecord)
- [sreportal.io/v1alpha1.FlowEdgeSet](#sreportaliov1alpha1flowedgeset)
- [sreportal.io/v1alpha1.FlowNodeSet](#sreportaliov1alpha1flownodeset)
- [sreportal.io/v1alpha1.FlowObserver](#sreportaliov1alpha1flowobserver)
- [sreportal.io/v1alpha1.ImageInventory](#sreportaliov1alpha1imageinventory)
- [sreportal.io/v1alpha1.Incident](#sreportaliov1alpha1incident)
- [sreportal.io/v1alpha1.Maintenance](#sreportaliov1alpha1maintenance)
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



#### sreportal.io/v1alpha1.Component

Component is the Schema for the components API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Component` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.ComponentSpec](#sreportaliov1alpha1componentspec)_ | spec defines the desired state of Component |   |   |
| `status` _[sreportal.io/v1alpha1.ComponentStatus](#sreportaliov1alpha1componentstatus)_ | status defines the observed state of Component |   |   |



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



#### sreportal.io/v1alpha1.FlowObserver

FlowObserver is the Schema for the flowobservers API. It configures how the operator detects real traffic on network edges by querying Prometheus for mesh/CNI flow metrics.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `FlowObserver` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.FlowObserverSpec](#sreportaliov1alpha1flowobserverspec)_ | spec defines the desired state of FlowObserver |   |   |
| `status` _[sreportal.io/v1alpha1.FlowObserverStatus](#sreportaliov1alpha1flowobserverstatus)_ | status defines the observed state of FlowObserver |   |   |



#### sreportal.io/v1alpha1.ImageInventory

ImageInventory is the Schema for the imageinventories API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `ImageInventory` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.ImageInventorySpec](#sreportaliov1alpha1imageinventoryspec)_ | spec defines the desired state of ImageInventory |   |   |
| `status` _[sreportal.io/v1alpha1.ImageInventoryStatus](#sreportaliov1alpha1imageinventorystatus)_ | status defines the observed state of ImageInventory |   |   |



#### sreportal.io/v1alpha1.Incident

Incident is the Schema for the incidents API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Incident` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.IncidentSpec](#sreportaliov1alpha1incidentspec)_ | spec defines the desired state of Incident |   |   |
| `status` _[sreportal.io/v1alpha1.IncidentStatus](#sreportaliov1alpha1incidentstatus)_ | status defines the observed state of Incident |   |   |



#### sreportal.io/v1alpha1.Maintenance

Maintenance is the Schema for the maintenances API

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sreportal.io/v1alpha1` |   |   |
| `kind` _string_ | `Maintenance` |   |   |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |   |   |
| `spec` _[sreportal.io/v1alpha1.MaintenanceSpec](#sreportaliov1alpha1maintenancespec)_ | spec defines the desired state of Maintenance |   |   |
| `status` _[sreportal.io/v1alpha1.MaintenanceStatus](#sreportaliov1alpha1maintenancestatus)_ | status defines the observed state of Maintenance |   |   |



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



#### sreportal.io/v1alpha1.ComponentSpec

ComponentSpec defines the desired state of Component

_Appears in:_
- [sreportal.io/v1alpha1.Component](#sreportaliov1alpha1component)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `displayName` _string_ | displayName is the human-readable name shown on the status page |   |   |
| `description` _string_ | description is a short text displayed below the component name |   |   |
| `group` _string_ | group is a logical grouping for the status page (e.g. "Infrastructure", "Applications") |   |   |
| `link` _string_ | link is an optional external URL (e.g. GCP console, Grafana dashboard) |   | Pattern: `^https?://.*` |
| `portalRef` _string_ | portalRef is the name of the Portal this component is linked to |   |   |
| `status` _[sreportal.io/v1alpha1.ComponentStatusValue](#sreportaliov1alpha1componentstatusvalue)_ | status is the manually declared operational status |   |   |



#### sreportal.io/v1alpha1.DailyComponentStatus

DailyComponentStatus records the worst observed status for a single UTC calendar day.
status is the manually declared operational status

_Appears in:_
- [sreportal.io/v1alpha1.ComponentStatus](#sreportaliov1alpha1componentstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `date` _string_ | date is the UTC calendar day in YYYY-MM-DD format. |   | Pattern: `^\d\{4\}-\d\{2\}-\d\{2\}$` |
| `worstStatus` _[sreportal.io/v1alpha1.ComputedComponentStatus](#sreportaliov1alpha1computedcomponentstatus)_ | worstStatus is the worst computed status observed during this day. |   |   |



#### sreportal.io/v1alpha1.ComponentStatus

ComponentStatus defines the observed state of Component.
worstStatus is the worst computed status observed during this day.

_Appears in:_
- [sreportal.io/v1alpha1.Component](#sreportaliov1alpha1component)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `computedStatus` _[sreportal.io/v1alpha1.ComputedComponentStatus](#sreportaliov1alpha1computedcomponentstatus)_ | computedStatus is the effective status calculated by the controller. If a maintenance is in progress on this component, it is overridden to "maintenance". Otherwise it reflects spec.status. |   |   |
| `activeIncidents` _integer_ | activeIncidents is the number of active (non-resolved) incidents linked to this component |   |   |
| `lastStatusChange` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastStatusChange is the timestamp of the last computedStatus transition |   |   |
| `dailyWorstStatus` _[sreportal.io/v1alpha1.DailyComponentStatus](#sreportaliov1alpha1dailycomponentstatus) array_ | dailyWorstStatus records the worst computed status per UTC day over a sliding window (30 days). |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Component resource. |   |   |



#### sreportal.io/v1alpha1.DNSSpec

DNSSpec defines the desired state of DNS

_Appears in:_
- [sreportal.io/v1alpha1.DNS](#sreportaliov1alpha1dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this DNS resource is linked to |   |   |
| `groups` _[sreportal.io/v1alpha1.DNSGroup](#sreportaliov1alpha1dnsgroup) array_ | groups is a list of DNS entry groups for organizing entries in the UI |   |   |
| `isRemote` _boolean_ | isRemote indicates this DNS resource is managed by the portal controller for a remote portal. When true, the DNS controller skips reconciliation and the portal controller manages the status directly. |   |   |



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



#### sreportal.io/v1alpha1.FlowObserverSpec

FlowObserverSpec defines the desired state of FlowObserver.

_Appears in:_
- [sreportal.io/v1alpha1.FlowObserver](#sreportaliov1alpha1flowobserver)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this observer is linked to |   |   |
| `reconcileInterval` _string_ | reconcileInterval is how often the observer queries for unused edges (e.g. "1m", "5m"). Defaults to the NetworkFlowDiscovery reconcile interval (1m) when empty. |   |   |
| `evaluatedEdgeTypes` _string array_ | evaluatedEdgeTypes is the list of node types whose edges the observer can evaluate. Only edges where both source and destination are of a listed type will be checked. Others are marked as not evaluated. Defaults to ["service"]. |   |   |
| `prometheus` _[sreportal.io/v1alpha1.FlowObserverPrometheusConfig](#sreportaliov1alpha1flowobserverprometheusconfig)_ | prometheus configures the Prometheus-based flow observation |   |   |
| `metrics` _[sreportal.io/v1alpha1.FlowMetricDescriptor](#sreportaliov1alpha1flowmetricdescriptor) array_ | metrics defines the list of mesh metric descriptors to probe. Each descriptor describes how a specific CNI/mesh exposes flow metrics in Prometheus. When empty, the built-in defaults (Hubble, Istio, Linkerd) are used. The observer probes each descriptor in order and uses the first one that returns results. |   |   |



#### sreportal.io/v1alpha1.FlowObserverPrometheusConfig

FlowObserverPrometheusConfig configures the Prometheus connection.

_Appears in:_
- [sreportal.io/v1alpha1.FlowObserverSpec](#sreportaliov1alpha1flowobserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | address is the Prometheus server URL |   |   |
| `queryWindow` _string_ | queryWindow is the PromQL range window for flow queries (e.g. "5m") |   |   |



#### sreportal.io/v1alpha1.FlowMetricDescriptor

FlowMetricDescriptor describes how a specific CNI or service mesh exposes flow metrics in Prometheus. The observer probes each descriptor in order and uses the first one whose probeQuery returns results.

_Appears in:_
- [sreportal.io/v1alpha1.FlowObserverSpec](#sreportaliov1alpha1flowobserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is a human-readable identifier for this mesh/CNI (e.g. "istio", "hubble", "linkerd") |   |   |
| `probeQuery` _string_ | probeQuery is a PromQL query used to detect if this mesh is active. Should return a non-empty vector when the mesh is present (e.g. "count(istio_requests_total)"). |   |   |
| `observedQueryTemplate` _string_ | observedQueryTemplate is a PromQL query template returning one result per source/destination pair. Use %s as placeholder for the query window (e.g. "5m"). Must use "max by" with the label names defined below. |   |   |
| `sourceNamespaceLabel` _string_ | sourceNamespaceLabel is the Prometheus label name for the source namespace |   |   |
| `sourceWorkloadLabel` _string_ | sourceWorkloadLabel is the Prometheus label name for the source workload |   |   |
| `destinationNamespaceLabel` _string_ | destinationNamespaceLabel is the Prometheus label name for the destination namespace |   |   |
| `destinationWorkloadLabel` _string_ | destinationWorkloadLabel is the Prometheus label name for the destination workload |   |   |



#### sreportal.io/v1alpha1.FlowObserverStatus

FlowObserverStatus defines the observed state of FlowObserver.
destinationWorkloadLabel is the Prometheus label name for the destination workload

_Appears in:_
- [sreportal.io/v1alpha1.FlowObserver](#sreportaliov1alpha1flowobserver)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `activeMesh` _string_ | activeMesh is the name of the detected mesh provider (empty if none detected) |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the FlowObserver resource. |   |   |



#### sreportal.io/v1alpha1.ImageInventorySpec

ImageInventorySpec defines the desired state of ImageInventory.

_Appears in:_
- [sreportal.io/v1alpha1.ImageInventory](#sreportaliov1alpha1imageinventory)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the Portal name this inventory belongs to. |   |   |
| `watchedKinds` _[sreportal.io/v1alpha1.ImageInventoryKind](#sreportaliov1alpha1imageinventorykind) array_ | watchedKinds declares which workload kinds are scanned for images. Empty means all supported defaults. |   |   |
| `namespaceFilter` _string_ | namespaceFilter restricts scan to a single namespace when set. Empty means all namespaces. |   |   |
| `labelSelector` _string_ | labelSelector is a Kubernetes label selector string used to filter workloads. Empty means no label filtering. |   |   |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | interval controls how often this inventory is refreshed. Empty means default 5m. |   |   |
| `isRemote` _boolean_ | isRemote marks this inventory as a shadow projection of a remote portal. When true, the controller fetches images from the remote portal via the ImageService Connect API instead of scanning the local cluster. |   |   |



#### sreportal.io/v1alpha1.ImageInventoryStatus

ImageInventoryStatus defines the observed state of ImageInventory.

_Appears in:_
- [sreportal.io/v1alpha1.ImageInventory](#sreportaliov1alpha1imageinventory)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | observedGeneration is the most recently observed generation. |   |   |
| `lastScanTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | lastScanTime is the timestamp of the latest completed scan. |   |   |
| `lastScanError` _string_ | lastScanError contains the latest scan error, if any. |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the ImageInventory resource. |   |   |



#### sreportal.io/v1alpha1.IncidentUpdate

IncidentUpdate represents a single timeline entry in the incident lifecycle.

_Appears in:_
- [sreportal.io/v1alpha1.IncidentSpec](#sreportaliov1alpha1incidentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `timestamp` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | timestamp is the time of this update |   |   |
| `phase` _[sreportal.io/v1alpha1.IncidentPhase](#sreportaliov1alpha1incidentphase)_ | phase is the incident phase at the time of this update |   |   |
| `message` _string_ | message is a human-readable description of the update |   |   |



#### sreportal.io/v1alpha1.IncidentSpec

IncidentSpec defines the desired state of Incident
message is a human-readable description of the update

_Appears in:_
- [sreportal.io/v1alpha1.Incident](#sreportaliov1alpha1incident)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `title` _string_ | title is the headline of the incident |   |   |
| `portalRef` _string_ | portalRef is the name of the Portal this incident is linked to |   |   |
| `components` _string array_ | components is the list of Component metadata.name values affected |   |   |
| `severity` _[sreportal.io/v1alpha1.IncidentSeverity](#sreportaliov1alpha1incidentseverity)_ | severity indicates the impact level of the incident |   |   |
| `updates` _[sreportal.io/v1alpha1.IncidentUpdate](#sreportaliov1alpha1incidentupdate) array_ | updates is the chronological timeline of the incident, appended via kubectl edit/patch |   |   |



#### sreportal.io/v1alpha1.IncidentStatus

IncidentStatus defines the observed state of Incident.

_Appears in:_
- [sreportal.io/v1alpha1.Incident](#sreportaliov1alpha1incident)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `currentPhase` _[sreportal.io/v1alpha1.IncidentPhase](#sreportaliov1alpha1incidentphase)_ | currentPhase is the phase from the most recent update (by timestamp) |   |   |
| `startedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | startedAt is the timestamp of the first update |   |   |
| `resolvedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | resolvedAt is the timestamp of the first update with phase=resolved |   |   |
| `durationMinutes` _integer_ | durationMinutes is the incident duration in minutes (computed when resolved) |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Incident resource. |   |   |



#### sreportal.io/v1alpha1.MaintenanceSpec

MaintenanceSpec defines the desired state of Maintenance

_Appears in:_
- [sreportal.io/v1alpha1.Maintenance](#sreportaliov1alpha1maintenance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `title` _string_ | title is the headline displayed on the status page |   |   |
| `description` _string_ | description is a longer explanation (markdown supported in the UI) |   |   |
| `portalRef` _string_ | portalRef is the name of the Portal this maintenance is linked to |   |   |
| `components` _string array_ | components is the list of Component metadata.name values affected by this maintenance |   |   |
| `scheduledStart` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | scheduledStart is the planned start time of the maintenance window |   |   |
| `scheduledEnd` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | scheduledEnd is the planned end time of the maintenance window |   |   |
| `affectedStatus` _[sreportal.io/v1alpha1.MaintenanceAffectedStatus](#sreportaliov1alpha1maintenanceaffectedstatus)_ | affectedStatus is the status applied to affected components during in_progress phase |   |   |



#### sreportal.io/v1alpha1.MaintenanceStatus

MaintenanceStatus defines the observed state of Maintenance.
affectedStatus is the status applied to affected components during in_progress phase

_Appears in:_
- [sreportal.io/v1alpha1.Maintenance](#sreportaliov1alpha1maintenance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[sreportal.io/v1alpha1.MaintenancePhase](#sreportaliov1alpha1maintenancephase)_ | phase is the lifecycle phase computed by the controller (upcoming, in_progress, completed) |   |   |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | conditions represent the current state of the Maintenance resource. |   |   |



#### sreportal.io/v1alpha1.NetworkFlowDiscoverySpec

NetworkFlowDiscoverySpec defines the desired state of NetworkFlowDiscovery.

_Appears in:_
- [sreportal.io/v1alpha1.NetworkFlowDiscovery](#sreportaliov1alpha1networkflowdiscovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this resource is linked to |   |   |
| `namespaces` _string array_ | namespaces is an optional list of namespaces to scan. When empty, all namespaces are scanned. |   |   |
| `isRemote` _boolean_ | isRemote indicates that the corresponding portal is remote and the operator should fetch network flows from the remote portal Connect API instead of scanning local Kubernetes NetworkPolicies. |   |   |



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
| `used` _boolean_ | used indicates whether traffic has been observed on this edge. |   |   |
| `evaluated` _boolean_ | evaluated indicates whether the observer attempted to check this edge. When false, the Used field should be ignored (edge type is not observable). |   |   |



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
| `features` _[sreportal.io/v1alpha1.PortalFeatures](#sreportaliov1alpha1portalfeatures)_ | features controls which features are enabled for this portal. All features default to true when not specified. |   |   |



#### sreportal.io/v1alpha1.PortalFeatures

PortalFeatures controls which features are enabled for a portal. All features default to true when not specified.

_Appears in:_
- [sreportal.io/v1alpha1.PortalSpec](#sreportaliov1alpha1portalspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dns` _boolean_ | dns enables DNS discovery (controllers, gRPC, MCP, web page) for this portal. |   |   |
| `releases` _boolean_ | releases enables the releases page for this portal. |   |   |
| `networkPolicy` _boolean_ | networkPolicy enables network policy visualization for this portal. |   |   |
| `alerts` _boolean_ | alerts enables alertmanager integration for this portal. |   |   |
| `statusPage` _boolean_ | statusPage enables the status page (components, incidents, maintenances) for this portal. |   |   |
| `imageInventory` _boolean_ | imageInventory enables the image inventory page for this portal. |   |   |



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
| `features` _[sreportal.io/v1alpha1.PortalFeaturesStatus](#sreportaliov1alpha1portalfeaturesstatus)_ | features contains the feature flags reported by the remote portal. Used to compute effective features for remote portals (local AND remote). |   |   |



#### sreportal.io/v1alpha1.PortalFeaturesStatus

PortalFeaturesStatus contains the observed feature flags from a remote portal. Unlike PortalFeatures (spec), these are explicit booleans with no nil-defaults-to-true semantics.

_Appears in:_
- [sreportal.io/v1alpha1.RemoteSyncStatus](#sreportaliov1alpha1remotesyncstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dns` _boolean_ | dns indicates whether the remote portal has DNS discovery enabled. |   |   |
| `releases` _boolean_ | releases indicates whether the remote portal has releases enabled. |   |   |
| `networkPolicy` _boolean_ | networkPolicy indicates whether the remote portal has network policy visualization enabled. |   |   |
| `alerts` _boolean_ | alerts indicates whether the remote portal has alertmanager integration enabled. |   |   |
| `statusPage` _boolean_ | statusPage indicates whether the remote portal has the status page enabled. |   |   |
| `imageInventory` _boolean_ | imageInventory indicates whether the remote portal has image inventory enabled. |   |   |



#### sreportal.io/v1alpha1.ReleaseSpec

ReleaseSpec defines the desired state of Release

_Appears in:_
- [sreportal.io/v1alpha1.Release](#sreportaliov1alpha1release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portalRef` _string_ | portalRef is the name of the Portal this release is linked to |   |   |
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





