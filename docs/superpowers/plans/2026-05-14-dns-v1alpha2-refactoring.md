# DNS v1alpha2 Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrer les CRDs `DNS` et `DNSRecord` vers v1alpha2, déplacer la configuration des sources dans le CRD DNS, et rendre `DNSRecord` dual-purpose (auto + manuel) en remplacement des `DNSEntry` inline.

**Architecture:** Le package `api/v1alpha2` devient la version de stockage (hub). `api/v1alpha1` implémente `ConvertTo`/`ConvertFrom`. Les controllers sont mis à jour pour utiliser v1alpha2. La config `sources/groupMapping/reconciliation` disparaît du ConfigMap et est lue depuis chaque DNS CR. Les entrées manuelles passent par des DNSRecord avec `spec.origin=manual`.

**Tech Stack:** Go 1.26, Kubebuilder v4, controller-runtime v0.23, Ginkgo v2 + Gomega, `k8s.io/apimachinery/pkg/conversion`, `metav1.Duration`

---

## Fichiers créés / modifiés

| Fichier | Action |
|---|---|
| `api/v1alpha2/groupversion_info.go` | Créer — registration du package v1alpha2 |
| `api/v1alpha2/dns_types.go` | Créer — nouveau schéma DNS avec Hub() |
| `api/v1alpha2/dnsrecord_types.go` | Créer — nouveau schéma DNSRecord avec Hub() |
| `api/v1alpha1/dns_types.go` | Modifier — ajouter ConvertTo/ConvertFrom |
| `api/v1alpha1/dnsrecord_types.go` | Modifier — ajouter ConvertTo/ConvertFrom |
| `internal/webhook/v1alpha2/dns_webhook.go` | Créer — validating: name=portalRef, immutabilité |
| `internal/webhook/v1alpha2/dnsrecord_webhook.go` | Créer — validating: Portal/DNS exists, origin=auto réservé |
| `internal/controller/dnsrecords/chain/chain_data.go` | Modifier — ajouter GroupMapping, DisableDNSCheck |
| `internal/controller/dnsrecords/chain/load_dns_config.go` | Créer — charge DNS CR v1alpha2 |
| `internal/controller/dnsrecords/chain/materialise_manual.go` | Créer — entries → endpoints |
| `internal/controller/dnsrecords/chain/project_store.go` | Modifier — groupMapping depuis ChainData, SourceManual |
| `internal/controller/dnsrecords/dnsrecord_controller.go` | Modifier — v1alpha2, nouveau constructeur, watch DNS |
| `internal/controller/dns/chain/` | Modifier — supprimer handlers manuels, ajouter aggregate |
| `internal/controller/dns/dns_controller.go` | Modifier — v1alpha2, simplifier chain |
| `internal/controller/source/source_controller.go` | Modifier — per-DNS config map, configChanged channel |
| `internal/controller/source/dns_config_notifier.go` | Créer — controller minimal de notification |
| `internal/adapter/adapter.go` | Modifier — GroupMappingConfig → v1alpha2.GroupMappingSpec |
| `cmd/main.go` | Modifier — v1alpha2 scheme, nouveaux constructeurs |
| `hack/migrate-dns-v2/main.go` | Créer — outil de migration |
| `config/samples/sreportal_v1alpha2_dns.yaml` | Créer |
| `config/samples/sreportal_v1alpha2_dnsrecord_manual.yaml` | Créer |

---

## Task 1: Package api/v1alpha2 — groupversion_info.go

**Files:**
- Create: `api/v1alpha2/groupversion_info.go`

- [ ] **Step 1: Créer le fichier**

```go
// api/v1alpha2/groupversion_info.go
package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "sreportal.io", Version: "v1alpha2"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)
```

- [ ] **Step 2: Vérifier compilation**

```bash
cd /Users/david/Projects/go/src/github.com/golgoth31/sreportal
go build ./api/v1alpha2/...
```
Expected: `build failed: no Go files` (normal, les types n'existent pas encore)

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha2/groupversion_info.go
git commit -m "feat(api): scaffold v1alpha2 package"
```

---

## Task 2: api/v1alpha2 — DNS types (hub)

**Files:**
- Create: `api/v1alpha2/dns_types.go`

- [ ] **Step 1: Créer les types**

```go
// api/v1alpha2/dns_types.go
package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSRecordOrigin discriminates auto-discovered vs manually created DNSRecord.
// +kubebuilder:validation:Enum=auto;manual
type DNSRecordOrigin string

const (
	DNSRecordOriginAuto   DNSRecordOrigin = "auto"
	DNSRecordOriginManual DNSRecordOrigin = "manual"
)

// SourcesSpec mirrors internal/config.SourcesConfig with Kubebuilder markers.
type SourcesSpec struct {
	Service                  *ServiceSourceSpec                  `json:"service,omitempty"`
	Ingress                  *IngressSourceSpec                  `json:"ingress,omitempty"`
	DNSEndpoint              *DNSEndpointSourceSpec              `json:"dnsEndpoint,omitempty"`
	IstioGateway             *IstioGatewaySourceSpec             `json:"istioGateway,omitempty"`
	IstioVirtualService      *IstioVirtualServiceSourceSpec      `json:"istioVirtualService,omitempty"`
	GatewayHTTPRoute         *GatewayRouteSourceSpec             `json:"gatewayHTTPRoute,omitempty"`
	GatewayGRPCRoute         *GatewayRouteSourceSpec             `json:"gatewayGRPCRoute,omitempty"`
	GatewayTLSRoute          *GatewayRouteSourceSpec             `json:"gatewayTLSRoute,omitempty"`
	GatewayTCPRoute          *GatewayRouteSourceSpec             `json:"gatewayTCPRoute,omitempty"`
	GatewayUDPRoute          *GatewayRouteSourceSpec             `json:"gatewayUDPRoute,omitempty"`
	CrossplaneScalewayRecord *CrossplaneScalewayRecordSourceSpec `json:"crossplaneScalewayRecord,omitempty"`
	// +optional
	Priority []string `json:"priority,omitempty"`
}

type ServiceSourceSpec struct {
	Enabled                  bool     `json:"enabled"`
	Namespace                string   `json:"namespace,omitempty"`
	AnnotationFilter         string   `json:"annotationFilter,omitempty"`
	LabelFilter              string   `json:"labelFilter,omitempty"`
	FQDNTemplate             string   `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool     `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool     `json:"ignoreHostnameAnnotation,omitempty"`
	PublishInternal          bool     `json:"publishInternal,omitempty"`
	PublishHostIP            bool     `json:"publishHostIP,omitempty"`
	ServiceTypeFilter        []string `json:"serviceTypeFilter,omitempty"`
}

type IngressSourceSpec struct {
	Enabled                  bool     `json:"enabled"`
	Namespace                string   `json:"namespace,omitempty"`
	AnnotationFilter         string   `json:"annotationFilter,omitempty"`
	LabelFilter              string   `json:"labelFilter,omitempty"`
	FQDNTemplate             string   `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool     `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool     `json:"ignoreHostnameAnnotation,omitempty"`
	IngressClassNames        []string `json:"ingressClassNames,omitempty"`
}

type DNSEndpointSourceSpec struct {
	Enabled     bool   `json:"enabled"`
	Namespace   string `json:"namespace,omitempty"`
	LabelFilter string `json:"labelFilter,omitempty"`
}

type IstioGatewaySourceSpec struct {
	Enabled                  bool   `json:"enabled"`
	Namespace                string `json:"namespace,omitempty"`
	AnnotationFilter         string `json:"annotationFilter,omitempty"`
	FQDNTemplate             string `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool   `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool   `json:"ignoreHostnameAnnotation,omitempty"`
}

type IstioVirtualServiceSourceSpec struct {
	Enabled                  bool   `json:"enabled"`
	Namespace                string `json:"namespace,omitempty"`
	AnnotationFilter         string `json:"annotationFilter,omitempty"`
	FQDNTemplate             string `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool   `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool   `json:"ignoreHostnameAnnotation,omitempty"`
}

type GatewayRouteSourceSpec struct {
	Enabled                  bool   `json:"enabled"`
	Namespace                string `json:"namespace,omitempty"`
	AnnotationFilter         string `json:"annotationFilter,omitempty"`
	LabelFilter              string `json:"labelFilter,omitempty"`
	FQDNTemplate             string `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool   `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool   `json:"ignoreHostnameAnnotation,omitempty"`
	GatewayName              string `json:"gatewayName,omitempty"`
	GatewayNamespace         string `json:"gatewayNamespace,omitempty"`
	GatewayLabelFilter       string `json:"gatewayLabelFilter,omitempty"`
}

type CrossplaneScalewayRecordSourceSpec struct {
	Enabled       bool   `json:"enabled"`
	Namespace     string `json:"namespace,omitempty"`
	LabelFilter   string `json:"labelFilter,omitempty"`
	ClusterScoped bool   `json:"clusterScoped,omitempty"`
}

// GroupMappingSpec configures how FQDNs are organised into groups in the UI.
type GroupMappingSpec struct {
	// +kubebuilder:default="Services"
	// +kubebuilder:validation:MinLength=1
	DefaultGroup string `json:"defaultGroup"`
	// +optional
	LabelKey string `json:"labelKey,omitempty"`
	// +optional
	ByNamespace map[string]string `json:"byNamespace,omitempty"`
}

// ReconciliationSpec controls timing of the source poll loop.
type ReconciliationSpec struct {
	// +kubebuilder:default="5m"
	Interval metav1.Duration `json:"interval"`
	// +kubebuilder:default="30s"
	RetryOnError    metav1.Duration `json:"retryOnError"`
	DisableDNSCheck bool            `json:"disableDNSCheck,omitempty"`
}

// DNSSpec defines the desired state of DNS (v1alpha2).
// DNS.metadata.name MUST equal spec.portalRef (enforced by webhook).
type DNSSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.portalRef is immutable"
	PortalRef string `json:"portalRef"`

	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// +optional
	Sources SourcesSpec `json:"sources,omitempty"`

	// +kubebuilder:default={defaultGroup:"Services"}
	// +optional
	GroupMapping GroupMappingSpec `json:"groupMapping,omitempty"`

	// +kubebuilder:default={interval:"5m",retryOnError:"30s"}
	// +optional
	Reconciliation ReconciliationSpec `json:"reconciliation,omitempty"`
}

// DNSStatus defines the observed state of DNS (v1alpha2).
type DNSStatus struct {
	Groups             []FQDNGroupStatus  `json:"groups,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastReconcileTime  *metav1.Time       `json:"lastReconcileTime,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	ActiveSources      []string           `json:"activeSources,omitempty"`
	NextReconcileTime  *metav1.Time       `json:"nextReconcileTime,omitempty"`
}

// FQDNGroupStatus, FQDNStatus — copier depuis api/v1alpha1/dns_types.go (types inchangés).

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dns,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type DNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              DNSSpec   `json:"spec"`
	Status            DNSStatus `json:"status,omitzero"`
}

func (*DNS) Hub() {}

// +kubebuilder:object:root=true
type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNS{}, &DNSList{})
}
```

Note: copier `FQDNGroupStatus` et `FQDNStatus` depuis `api/v1alpha1/dns_types.go` — ces types de status sont inchangés.

- [ ] **Step 2: Vérifier compilation**

```bash
go build ./api/v1alpha2/...
```
Expected: PASS (le deepcopy sera généré plus tard par `make generate`)

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha2/dns_types.go
git commit -m "feat(api/v1alpha2): add DNS types (hub version)"
```

---

## Task 3: api/v1alpha2 — DNSRecord types (hub)

**Files:**
- Create: `api/v1alpha2/dnsrecord_types.go`

- [ ] **Step 1: Créer les types**

```go
// api/v1alpha2/dnsrecord_types.go
package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSRecordSpec defines the desired state of DNSRecord (v1alpha2).
// +kubebuilder:validation:XValidation:rule="self.origin == 'auto' ? has(self.sourceType) && (!has(self.entries) || size(self.entries) == 0) : !has(self.sourceType) && has(self.entries) && size(self.entries) > 0",message="auto records require sourceType and no entries; manual records require entries and no sourceType"
type DNSRecordSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=auto;manual
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.origin is immutable"
	Origin DNSRecordOrigin `json:"origin"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.portalRef is immutable"
	PortalRef string `json:"portalRef"`

	// Required when origin=auto. Must be empty when origin=manual.
	// +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
	// +optional
	SourceType string `json:"sourceType,omitempty"`

	// Required when origin=manual. Must be empty when origin=auto.
	// +optional
	// +listType=map
	// +listMapKey=fqdn
	Entries []DNSRecordEntry `json:"entries,omitempty"`
}

// DNSRecordEntry is a single manual DNS entry.
type DNSRecordEntry struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	FQDN string `json:"fqdn"`

	// +optional
	Group string `json:"group,omitempty"`

	// +optional
	Description string `json:"description,omitempty"`

	// +kubebuilder:validation:Enum=A;AAAA;CNAME;TXT
	// +kubebuilder:default="A"
	// +optional
	RecordType string `json:"recordType,omitempty"`

	// +optional
	Targets []string `json:"targets,omitempty"`
}

// DNSRecordStatus defines the observed state of DNSRecord (v1alpha2).
// For origin=auto: Endpoints filled by SourceReconciler.
// For origin=manual: Endpoints filled by DNSRecordReconciler from spec.entries.
type DNSRecordStatus struct {
	Endpoints         []EndpointStatus   `json:"endpoints,omitempty"`
	EndpointsHash     string             `json:"endpointsHash,omitempty"`
	LastReconcileTime *metav1.Time       `json:"lastReconcileTime,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration,omitempty"`
}

// EndpointStatus — copier depuis api/v1alpha1/dnsrecord_types.go (inchangé).

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dnsrecords,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Origin",type=string,JSONPath=`.spec.origin`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              DNSRecordSpec   `json:"spec"`
	Status            DNSRecordStatus `json:"status,omitzero"`
}

func (*DNSRecord) Hub() {}

// +kubebuilder:object:root=true
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNSRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
```

Note: copier `EndpointStatus` depuis `api/v1alpha1/dnsrecord_types.go`.

- [ ] **Step 2: Vérifier compilation**

```bash
go build ./api/v1alpha2/...
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha2/dnsrecord_types.go
git commit -m "feat(api/v1alpha2): add DNSRecord types (hub version)"
```

---

## Task 4: Conversion DNS v1alpha1 ↔ v1alpha2

**Files:**
- Modify: `api/v1alpha1/dns_types.go`

- [ ] **Step 1: Écrire le test de conversion**

```go
// api/v1alpha1/dns_conversion_test.go
package v1alpha1_test

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "github.com/onsi/gomega"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

func TestDNSConvertTo_PreservesGroups(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha1.DNS{
		Spec: v1alpha1.DNSSpec{
			PortalRef: "main",
			Groups: []v1alpha1.DNSGroup{
				{
					Name: "APIs",
					Entries: []v1alpha1.DNSEntry{
						{FQDN: "api.example.com", Description: "Main API"},
					},
				},
			},
		},
	}

	dst := &v1alpha2.DNS{}
	err := src.ConvertTo(dst)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dst.Spec.PortalRef).To(Equal("main"))

	// groups preserved in annotation
	raw := dst.Annotations["sreportal.io/v1alpha1-groups"]
	g.Expect(raw).NotTo(BeEmpty())
	var groups []v1alpha1.DNSGroup
	g.Expect(json.Unmarshal([]byte(raw), &groups)).To(Succeed())
	g.Expect(groups).To(HaveLen(1))
	g.Expect(groups[0].Name).To(Equal("APIs"))
}

func TestDNSConvertFrom_RestoresGroups(t *testing.T) {
	g := NewWithT(t)

	groups := []v1alpha1.DNSGroup{{Name: "APIs", Entries: []v1alpha1.DNSEntry{{FQDN: "api.example.com"}}}}
	raw, _ := json.Marshal(groups)

	src := &v1alpha2.DNS{
		Spec: v1alpha2.DNSSpec{PortalRef: "main"},
	}
	src.Annotations = map[string]string{"sreportal.io/v1alpha1-groups": string(raw)}

	dst := &v1alpha1.DNS{}
	err := dst.ConvertFrom(src)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dst.Spec.PortalRef).To(Equal("main"))
	g.Expect(dst.Spec.Groups).To(HaveLen(1))
	g.Expect(dst.Spec.Groups[0].Name).To(Equal("APIs"))
}
```

- [ ] **Step 2: Lancer le test — vérifier qu'il échoue**

```bash
go test ./api/v1alpha1/... -run TestDNSConvert -v
```
Expected: FAIL — `ConvertTo` undefined

- [ ] **Step 3: Implémenter ConvertTo/ConvertFrom dans dns_types.go**

Ajouter à la fin de `api/v1alpha1/dns_types.go` :

```go
import (
    "encoding/json"
    "sigs.k8s.io/controller-runtime/pkg/conversion"
    v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const annotationV1Alpha1Groups = "sreportal.io/v1alpha1-groups"

// ConvertTo converts this DNS (v1alpha1) to the Hub version (v1alpha2).
func (src *DNS) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1alpha2.DNS)
    dst.ObjectMeta = src.ObjectMeta

    dst.Spec.PortalRef = src.Spec.PortalRef
    dst.Spec.IsRemote = src.Spec.IsRemote
    // sources/groupMapping/reconciliation left empty — migration tool fills them

    if len(src.Spec.Groups) > 0 {
        raw, err := json.Marshal(src.Spec.Groups)
        if err != nil {
            return err
        }
        if dst.Annotations == nil {
            dst.Annotations = make(map[string]string)
        }
        dst.Annotations[annotationV1Alpha1Groups] = string(raw)
    }

    dst.Status.Groups = src.Status.Groups
    dst.Status.Conditions = src.Status.Conditions
    dst.Status.LastReconcileTime = src.Status.LastReconcileTime
    return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this DNS (v1alpha1).
func (dst *DNS) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*v1alpha2.DNS)
    dst.ObjectMeta = src.ObjectMeta

    dst.Spec.PortalRef = src.Spec.PortalRef
    dst.Spec.IsRemote = src.Spec.IsRemote

    if raw, ok := src.Annotations[annotationV1Alpha1Groups]; ok && raw != "" {
        var groups []DNSGroup
        if err := json.Unmarshal([]byte(raw), &groups); err != nil {
            return err
        }
        dst.Spec.Groups = groups
    }

    dst.Status.Groups = src.Status.Groups
    dst.Status.Conditions = src.Status.Conditions
    dst.Status.LastReconcileTime = src.Status.LastReconcileTime
    return nil
}
```

- [ ] **Step 4: Lancer le test**

```bash
go test ./api/v1alpha1/... -run TestDNSConvert -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/dns_types.go api/v1alpha1/dns_conversion_test.go
git commit -m "feat(api/v1alpha1): implement DNS conversion to/from v1alpha2"
```

---

## Task 5: Conversion DNSRecord v1alpha1 ↔ v1alpha2

**Files:**
- Modify: `api/v1alpha1/dnsrecord_types.go`

- [ ] **Step 1: Écrire le test**

```go
// api/v1alpha1/dnsrecord_conversion_test.go
package v1alpha1_test

import (
	"testing"
	. "github.com/onsi/gomega"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

func TestDNSRecordConvertTo_SetsOriginAuto(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha1.DNSRecord{
		Spec: v1alpha1.DNSRecordSpec{
			PortalRef:  "main",
			SourceType: "ingress",
		},
	}
	dst := &v1alpha2.DNSRecord{}
	g.Expect(src.ConvertTo(dst)).To(Succeed())
	g.Expect(dst.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginAuto))
	g.Expect(dst.Spec.PortalRef).To(Equal("main"))
	g.Expect(dst.Spec.SourceType).To(Equal("ingress"))
}

func TestDNSRecordConvertFrom_PreservesOriginAuto(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  "main",
			SourceType: "service",
		},
	}
	dst := &v1alpha1.DNSRecord{}
	g.Expect(dst.ConvertFrom(src)).To(Succeed())
	g.Expect(dst.Spec.PortalRef).To(Equal("main"))
	g.Expect(dst.Spec.SourceType).To(Equal("service"))
}
```

- [ ] **Step 2: Lancer le test — vérifier qu'il échoue**

```bash
go test ./api/v1alpha1/... -run TestDNSRecordConvert -v
```
Expected: FAIL — `ConvertTo` undefined

- [ ] **Step 3: Implémenter ConvertTo/ConvertFrom**

Ajouter à la fin de `api/v1alpha1/dnsrecord_types.go` :

```go
import (
    "sigs.k8s.io/controller-runtime/pkg/conversion"
    v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// ConvertTo converts this DNSRecord (v1alpha1) to the Hub version (v1alpha2).
// All existing v1alpha1 records are auto-discovered, so origin=auto.
func (src *DNSRecord) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1alpha2.DNSRecord)
    dst.ObjectMeta = src.ObjectMeta
    dst.Spec.Origin = v1alpha2.DNSRecordOriginAuto
    dst.Spec.PortalRef = src.Spec.PortalRef
    dst.Spec.SourceType = src.Spec.SourceType
    dst.Status.Endpoints = src.Status.Endpoints
    dst.Status.EndpointsHash = src.Status.EndpointsHash
    dst.Status.LastReconcileTime = src.Status.LastReconcileTime
    dst.Status.Conditions = src.Status.Conditions
    return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this DNSRecord (v1alpha1).
// Manual-only fields (entries) are lost — v1alpha1 does not support them.
func (dst *DNSRecord) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*v1alpha2.DNSRecord)
    dst.ObjectMeta = src.ObjectMeta
    dst.Spec.PortalRef = src.Spec.PortalRef
    dst.Spec.SourceType = src.Spec.SourceType
    dst.Status.Endpoints = src.Status.Endpoints
    dst.Status.EndpointsHash = src.Status.EndpointsHash
    dst.Status.LastReconcileTime = src.Status.LastReconcileTime
    dst.Status.Conditions = src.Status.Conditions
    return nil
}
```

- [ ] **Step 4: Lancer les tests**

```bash
go test ./api/v1alpha1/... -run TestDNSRecordConvert -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/dnsrecord_types.go api/v1alpha1/dnsrecord_conversion_test.go
git commit -m "feat(api/v1alpha1): implement DNSRecord conversion to/from v1alpha2"
```

---

## Task 6: Webhook validating DNS v1alpha2

**Files:**
- Create: `internal/webhook/v1alpha2/dns_webhook.go`

- [ ] **Step 1: Écrire les tests**

```go
// internal/webhook/v1alpha2/dns_webhook_test.go
package v1alpha2_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	webhookv1alpha2 "github.com/golgoth31/sreportal/internal/webhook/v1alpha2"
)

func TestDNSWebhook_NameMustEqualPortalRef(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator(nil)
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main"},
		Spec:       v1alpha2.DNSSpec{PortalRef: "other"},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("name must equal portalRef"))
}

func TestDNSWebhook_ValidCreate(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator(nil)
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main"},
		Spec:       v1alpha2.DNSSpec{PortalRef: "main"},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSWebhook_PortalRefImmutable(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator(nil)
	old := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main"},
		Spec:       v1alpha2.DNSSpec{PortalRef: "main"},
	}
	new := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main"},
		Spec:       v1alpha2.DNSSpec{PortalRef: "other"},
	}
	_, err := v.ValidateUpdate(context.Background(), old, new)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("portalRef is immutable"))
}
```

- [ ] **Step 2: Lancer les tests — vérifier qu'ils échouent**

```bash
go test ./internal/webhook/v1alpha2/... -v
```
Expected: FAIL — package not found

- [ ] **Step 3: Implémenter le webhook**

```go
// internal/webhook/v1alpha2/dns_webhook.go
package v1alpha2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha2-dns,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=dns,verbs=create;update,versions=v1alpha2,name=vdns-v1alpha2.kb.io,admissionReviewVersions=v1

type DNSCustomValidator struct {
	client client.Client
}

func NewDNSCustomValidator(c client.Client) *DNSCustomValidator {
	return &DNSCustomValidator{client: c}
}

func (v *DNSCustomValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha2.DNS{}).
		WithValidator(v).
		Complete()
}

func (v *DNSCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dns := obj.(*v1alpha2.DNS)
	if dns.Name != dns.Spec.PortalRef {
		return nil, fmt.Errorf("name must equal portalRef: got name=%q portalRef=%q", dns.Name, dns.Spec.PortalRef)
	}
	return nil, nil
}

func (v *DNSCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old := oldObj.(*v1alpha2.DNS)
	new := newObj.(*v1alpha2.DNS)
	if old.Spec.PortalRef != new.Spec.PortalRef {
		return nil, fmt.Errorf("portalRef is immutable: cannot change from %q to %q", old.Spec.PortalRef, new.Spec.PortalRef)
	}
	if new.Name != new.Spec.PortalRef {
		return nil, fmt.Errorf("name must equal portalRef: got name=%q portalRef=%q", new.Name, new.Spec.PortalRef)
	}
	return nil, nil
}

func (v *DNSCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
```

- [ ] **Step 4: Lancer les tests**

```bash
go test ./internal/webhook/v1alpha2/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/v1alpha2/
git commit -m "feat(webhook/v1alpha2): add DNS validating webhook"
```

---

## Task 7: Webhook validating DNSRecord v1alpha2

**Files:**
- Create: `internal/webhook/v1alpha2/dnsrecord_webhook.go`

- [ ] **Step 1: Écrire les tests**

```go
// internal/webhook/v1alpha2/dnsrecord_webhook_test.go
package v1alpha2_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	webhookv1alpha2 "github.com/golgoth31/sreportal/internal/webhook/v1alpha2"
)

func TestDNSRecordWebhook_AutoRequiresSourceType(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(nil, "system:serviceaccount:sreportal-system:sreportal-controller-manager")
	r := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-ingress"},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginAuto,
			PortalRef: "main",
			// sourceType missing
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
}

func TestDNSRecordWebhook_ManualRequiresEntries(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(nil, "")
	r := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-manual-apis"},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: "main",
			// entries empty
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("entries"))
}

func TestDNSRecordWebhook_ManualValidCreate(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(nil, "")
	r := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-manual-apis"},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: "main",
			Entries:   []v1alpha2.DNSRecordEntry{{FQDN: "api.example.com"}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).NotTo(HaveOccurred())
}
```

- [ ] **Step 2: Lancer les tests — vérifier qu'ils échouent**

```bash
go test ./internal/webhook/v1alpha2/... -run TestDNSRecord -v
```
Expected: FAIL

- [ ] **Step 3: Implémenter le webhook**

```go
// internal/webhook/v1alpha2/dnsrecord_webhook.go
package v1alpha2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha2-dnsrecord,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=dnsrecords,verbs=create;update,versions=v1alpha2,name=vdnsrecord-v1alpha2.kb.io,admissionReviewVersions=v1

type DNSRecordCustomValidator struct {
	client        client.Client
	controllerSA  string // e.g. "system:serviceaccount:sreportal-system:sreportal-controller-manager"
}

func NewDNSRecordCustomValidator(c client.Client, controllerSA string) *DNSRecordCustomValidator {
	return &DNSRecordCustomValidator{client: c, controllerSA: controllerSA}
}

func (v *DNSRecordCustomValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha2.DNSRecord{}).
		WithValidator(v).
		Complete()
}

func (v *DNSRecordCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	r := obj.(*v1alpha2.DNSRecord)
	return nil, v.validate(ctx, r, nil)
}

func (v *DNSRecordCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	r := newObj.(*v1alpha2.DNSRecord)
	return nil, v.validate(ctx, r, oldObj.(*v1alpha2.DNSRecord))
}

func (v *DNSRecordCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *DNSRecordCustomValidator) validate(ctx context.Context, r *v1alpha2.DNSRecord, old *v1alpha2.DNSRecord) error {
	if r.Spec.Origin == v1alpha2.DNSRecordOriginAuto {
		if r.Spec.SourceType == "" {
			return fmt.Errorf("spec.sourceType is required when spec.origin=auto")
		}
		if len(r.Spec.Entries) > 0 {
			return fmt.Errorf("spec.entries must be empty when spec.origin=auto")
		}
		// origin=auto reserved to controller SA
		if v.controllerSA != "" {
			req, err := admission.RequestFromContext(ctx)
			if err == nil && req.UserInfo.Username != v.controllerSA {
				return fmt.Errorf("spec.origin=auto is reserved for the operator controller")
			}
		}
	}
	if r.Spec.Origin == v1alpha2.DNSRecordOriginManual {
		if r.Spec.SourceType != "" {
			return fmt.Errorf("spec.sourceType must be empty when spec.origin=manual")
		}
		if len(r.Spec.Entries) == 0 {
			return fmt.Errorf("spec.entries must have at least one entry when spec.origin=manual")
		}
	}
	if old != nil && old.Spec.Origin != r.Spec.Origin {
		return fmt.Errorf("spec.origin is immutable")
	}
	if old != nil && old.Spec.PortalRef != r.Spec.PortalRef {
		return fmt.Errorf("spec.portalRef is immutable")
	}
	return nil
}
```

- [ ] **Step 4: Lancer les tests**

```bash
go test ./internal/webhook/v1alpha2/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/v1alpha2/dnsrecord_webhook.go internal/webhook/v1alpha2/dnsrecord_webhook_test.go
git commit -m "feat(webhook/v1alpha2): add DNSRecord validating webhook"
```

---

## Task 8: Générer CRDs et deepcopy

**Files:**
- Modified (generated): `config/crd/bases/`, `api/v1alpha1/zz_generated.deepcopy.go`, `api/v1alpha2/zz_generated.deepcopy.go`

- [ ] **Step 1: Enregistrer v1alpha2 dans le scheme (cmd/main.go)**

Dans `cmd/main.go`, ajouter l'import et l'enregistrement du scheme v1alpha2 :

```go
// Dans la section imports
sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"

// Dans init() ou dans la fonction main() avant mgr creation
// utiliser: utilityscheme.AddToScheme = func(s *runtime.Scheme) error {
//   if err := sreportalv1alpha1.AddToScheme(s); err != nil { return err }
//   return sreportalv1alpha2.AddToScheme(s)
// }
```

Trouver la section où les schemes sont enregistrés dans `cmd/main.go` (chercher `AddToScheme`) et ajouter :
```go
utilruntime.Must(sreportalv1alpha2.AddToScheme(scheme))
```

- [ ] **Step 2: Lancer make manifests generate**

```bash
make manifests generate
```
Expected: génère `config/crd/bases/sreportal.io_dns.yaml` (multi-version v1alpha1+v1alpha2), `config/crd/bases/sreportal.io_dnsrecords.yaml`, et `api/v1alpha2/zz_generated.deepcopy.go`

- [ ] **Step 3: Vérifier la compilation globale**

```bash
go build ./...
```
Expected: PASS (quelques erreurs possibles sur les imports v1alpha2 non encore utilisés — acceptable)

- [ ] **Step 4: Commit**

```bash
git add config/crd/ api/v1alpha1/zz_generated.deepcopy.go api/v1alpha2/ cmd/main.go
git commit -m "chore: regenerate CRDs and deepcopy for v1alpha2"
```

---

## Task 9: ChainData DNSRecord + LoadDNSConfigHandler

**Files:**
- Modify: `internal/controller/dnsrecords/chain/chain_data.go` (ou fichier contenant `ChainData`)
- Create: `internal/controller/dnsrecords/chain/load_dns_config.go`

- [ ] **Step 1: Écrire le test**

```go
// internal/controller/dnsrecords/chain/load_dns_config_test.go
package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestLoadDNSConfigHandler_PopulatesChainData(t *testing.T) {
	g := NewWithT(t)

	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: v1alpha2.DNSSpec{
			PortalRef: "main",
			GroupMapping: v1alpha2.GroupMappingSpec{
				DefaultGroup: "MyServices",
			},
			Reconciliation: v1alpha2.ReconciliationSpec{
				DisableDNSCheck: true,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithObjects(dns).
		Build()

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  "main",
			SourceType: "ingress",
		},
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: "default/main-ingress"},
	}

	h := chain.NewLoadDNSConfigHandler(c)
	err := h.Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rc.Data.GroupMapping).NotTo(BeNil())
	g.Expect(rc.Data.GroupMapping.DefaultGroup).To(Equal("MyServices"))
	g.Expect(rc.Data.DisableDNSCheck).To(BeTrue())
}
```

- [ ] **Step 2: Lancer le test — vérifier qu'il échoue**

```bash
go test ./internal/controller/dnsrecords/chain/... -run TestLoadDNSConfig -v
```
Expected: FAIL

- [ ] **Step 3: Étendre ChainData**

Dans le fichier contenant `ChainData` du package `dnsrecords/chain` (probablement `chain_data.go` ou dans un fichier existant), modifier :

```go
// internal/controller/dnsrecords/chain/chain_data.go
package chain

import v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"

type ChainData struct {
    ResourceKey     string
    GroupMapping    *v1alpha2.GroupMappingSpec
    DisableDNSCheck bool
}
```

- [ ] **Step 4: Créer LoadDNSConfigHandler**

```go
// internal/controller/dnsrecords/chain/load_dns_config.go
package chain

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type LoadDNSConfigHandler struct {
	client client.Client
}

func NewLoadDNSConfigHandler(c client.Client) *LoadDNSConfigHandler {
	return &LoadDNSConfigHandler{client: c}
}

func (h *LoadDNSConfigHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	record := rc.Resource
	var dns v1alpha2.DNS
	if err := h.client.Get(ctx, client.ObjectKey{
		Name:      record.Spec.PortalRef,
		Namespace: record.Namespace,
	}, &dns); err != nil {
		return fmt.Errorf("load DNS config for portal %q: %w", record.Spec.PortalRef, err)
	}
	rc.Data.GroupMapping = &dns.Spec.GroupMapping
	rc.Data.DisableDNSCheck = dns.Spec.Reconciliation.DisableDNSCheck
	return nil
}
```

- [ ] **Step 5: Lancer le test**

```bash
go test ./internal/controller/dnsrecords/chain/... -run TestLoadDNSConfig -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/controller/dnsrecords/chain/
git commit -m "feat(dnsrecords/chain): add LoadDNSConfigHandler and extend ChainData"
```

---

## Task 10: MaterialiseManualEntriesHandler

**Files:**
- Create: `internal/controller/dnsrecords/chain/materialise_manual.go`

- [ ] **Step 1: Écrire le test**

```go
// internal/controller/dnsrecords/chain/materialise_manual_test.go
package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestMaterialiseManualEntriesHandler_ConvertEntriesToEndpoints(t *testing.T) {
	g := NewWithT(t)

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-manual-apis", Namespace: "default"},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: "main",
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "api.example.com", Group: "APIs", RecordType: "A", Targets: []string{"1.2.3.4"}},
				{FQDN: "graphql.example.com", Group: "APIs"},
			},
		},
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: "default/main-manual-apis"},
	}

	h := chain.NewMaterialiseManualEntriesHandler()
	err := h.Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(record.Status.Endpoints).To(HaveLen(2))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("api.example.com"))
	g.Expect(record.Status.Endpoints[0].RecordType).To(Equal("A"))
	g.Expect(record.Status.Endpoints[0].Targets).To(ConsistOf("1.2.3.4"))
	g.Expect(record.Status.Endpoints[1].DNSName).To(Equal("graphql.example.com"))
	g.Expect(record.Status.Endpoints[1].RecordType).To(Equal("A")) // default
}

func TestMaterialiseManualEntriesHandler_NoopForAuto(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			SourceType: "ingress",
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseManualEntriesHandler()
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(BeNil())
}
```

- [ ] **Step 2: Lancer le test — vérifier qu'il échoue**

```bash
go test ./internal/controller/dnsrecords/chain/... -run TestMaterialise -v
```
Expected: FAIL

- [ ] **Step 3: Implémenter le handler**

```go
// internal/controller/dnsrecords/chain/materialise_manual.go
package chain

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type MaterialiseManualEntriesHandler struct{}

func NewMaterialiseManualEntriesHandler() *MaterialiseManualEntriesHandler {
	return &MaterialiseManualEntriesHandler{}
}

func (h *MaterialiseManualEntriesHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	if rc.Resource.Spec.Origin != v1alpha2.DNSRecordOriginManual {
		return nil
	}
	now := metav1.Now()
	endpoints := make([]v1alpha2.EndpointStatus, 0, len(rc.Resource.Spec.Entries))
	for _, e := range rc.Resource.Spec.Entries {
		rt := e.RecordType
		if rt == "" {
			rt = "A"
		}
		labels := map[string]string{}
		if e.Group != "" {
			labels["sreportal.io/group"] = e.Group
		}
		endpoints = append(endpoints, v1alpha2.EndpointStatus{
			DNSName:    e.FQDN,
			RecordType: rt,
			Targets:    e.Targets,
			Labels:     labels,
		})
	}
	rc.Resource.Status.Endpoints = endpoints
	rc.Resource.Status.LastReconcileTime = &now
	return nil
}
```

- [ ] **Step 4: Lancer les tests**

```bash
go test ./internal/controller/dnsrecords/chain/... -run TestMaterialise -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controller/dnsrecords/chain/materialise_manual.go internal/controller/dnsrecords/chain/materialise_manual_test.go
git commit -m "feat(dnsrecords/chain): add MaterialiseManualEntriesHandler"
```

---

## Task 11: Adapter ProjectStoreHandler + adapter pour v1alpha2

**Files:**
- Modify: `internal/controller/dnsrecords/chain/project_store.go`
- Modify: `internal/adapter/adapter.go` (changer signature `GroupMappingConfig` → `v1alpha2.GroupMappingSpec`)

- [ ] **Step 1: Mettre à jour adapter.go**

Dans `internal/adapter/adapter.go`, changer la signature de `EndpointsToGroups` et `EndpointsToGroups`-related functions :

```go
// Remplacer: func EndpointsToGroups(eps []*endpoint.Endpoint, mapping *config.GroupMappingConfig) ...
// Par:
import v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"

func EndpointsToGroups(eps []*endpoint.Endpoint, mapping *v1alpha2.GroupMappingSpec) []FQDNGroupView {
    defaultGroup := "Services"
    if mapping != nil && mapping.DefaultGroup != "" {
        defaultGroup = mapping.DefaultGroup
    }
    // ... reste de l'implémentation identique, remplacer mapping.DefaultGroup, mapping.LabelKey, mapping.ByNamespace
}
```

Supprimer l'import `"github.com/golgoth31/sreportal/internal/config"` de l'adapter s'il n'est plus utilisé.

- [ ] **Step 2: Mettre à jour ProjectStoreHandler**

```go
// internal/controller/dnsrecords/chain/project_store.go
// Remplacer groupMapping *config.GroupMappingConfig par lecture depuis ChainData

type ProjectStoreHandler struct {
	fqdnWriter domaindns.FQDNWriter
}

func NewProjectStoreHandler(w domaindns.FQDNWriter) *ProjectStoreHandler {
	return &ProjectStoreHandler{fqdnWriter: w}
}

func (h *ProjectStoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	if h.fqdnWriter == nil {
		return nil
	}
	views := DNSRecordToFQDNViews(rc.Resource, rc.Data.GroupMapping)
	if err := h.fqdnWriter.Replace(ctx, rc.Data.ResourceKey, views); err != nil {
		log.FromContext(ctx).Error(err, "failed to update FQDN read store")
	}
	return nil
}
```

Mettre à jour `DNSRecordToFQDNViews` pour utiliser `*v1alpha2.GroupMappingSpec` et positionner `domaindns.SourceManual` quand `record.Spec.Origin == v1alpha2.DNSRecordOriginManual`.

- [ ] **Step 3: Vérifier compilation**

```bash
go build ./internal/adapter/... ./internal/controller/dnsrecords/...
```

- [ ] **Step 4: Mettre à jour les tests adapter**

Dans `internal/adapter/adapter_test.go`, remplacer les occurrences de `config.GroupMappingConfig` par `v1alpha2.GroupMappingSpec`.

- [ ] **Step 5: Lancer les tests**

```bash
go test ./internal/adapter/... ./internal/controller/dnsrecords/chain/... -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/ internal/controller/dnsrecords/chain/project_store.go
git commit -m "refactor(dnsrecords): migrate ProjectStoreHandler and adapter to v1alpha2 types"
```

---

## Task 12: Adapter DNSRecordReconciler

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go`

- [ ] **Step 1: Mettre à jour le constructeur**

```go
// Remplacer:
func NewDNSRecordReconciler(
    c client.Client,
    scheme *runtime.Scheme,
    groupMapping *config.GroupMappingConfig,
    resolver domaindns.Resolver,
    disableDNSCheck bool,
) *DNSRecordReconciler

// Par:
func NewDNSRecordReconciler(
    c client.Client,
    scheme *runtime.Scheme,
    resolver domaindns.Resolver,
) *DNSRecordReconciler
```

Supprimer les champs `groupMapping` et `disableDNSCheck` du struct (ils sont maintenant dans ChainData).

- [ ] **Step 2: Mettre à jour rebuildChain**

```go
func (r *DNSRecordReconciler) rebuildChain() {
    r.chain = reconciler.NewChain(
        "dnsrecord",
        dnsrecordchain.NewLoadDNSConfigHandler(r.Client),
        dnsrecordchain.NewMaterialiseManualEntriesHandler(),
        dnsrecordchain.NewSyncEndpointsHashHandler(r.Client),
        dnsrecordchain.NewResolveDNSHandler(r.Client, r.resolver), // lit DisableDNSCheck depuis ChainData
        dnsrecordchain.NewProjectStoreHandler(r.fqdnWriter),
    )
}
```

- [ ] **Step 3: Mettre à jour SetupWithManager — ajouter watch DNS**

```go
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha2.DNSRecord{}).
        Watches(
            &v1alpha2.DNS{},
            handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
                dns, ok := obj.(*v1alpha2.DNS)
                if !ok {
                    return nil
                }
                var list v1alpha2.DNSRecordList
                if err := r.Client.List(ctx, &list, client.InNamespace(dns.Namespace),
                    client.MatchingFields{"spec.portalRef": dns.Name}); err != nil {
                    return nil
                }
                reqs := make([]ctrl.Request, len(list.Items))
                for i, item := range list.Items {
                    reqs[i] = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&item)}
                }
                return reqs
            }),
        ).
        Named("dnsrecord").
        Complete(r)
}
```

- [ ] **Step 4: Mettre à jour ResolveDNSHandler pour lire DisableDNSCheck depuis ChainData**

Dans `internal/controller/dnsrecords/chain/resolve_dns.go` (ou équivalent), le handler doit lire `rc.Data.DisableDNSCheck` plutôt que son propre champ. Adapter la signature si nécessaire.

- [ ] **Step 5: Vérifier compilation**

```bash
go build ./internal/controller/dnsrecords/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/controller/dnsrecords/
git commit -m "refactor(dnsrecords): remove config dependency, add DNS watch, chain v1alpha2"
```

---

## Task 13: Simplifier DNS chain + DNSReconciler

**Files:**
- Modify: `internal/controller/dns/chain/*.go`
- Modify: `internal/controller/dns/dns_controller.go`

- [ ] **Step 1: Créer AggregateFromDNSRecordsHandler**

Ce handler remplace `CollectManualEntriesHandler` + `AggregateFQDNsHandler`. Il liste les `DNSRecord` du portal et construit les groups de status.

```go
// internal/controller/dns/chain/aggregate_dnsrecords.go
package chain

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type AggregateFromDNSRecordsHandler struct {
	client client.Client
}

func NewAggregateFromDNSRecordsHandler(c client.Client) *AggregateFromDNSRecordsHandler {
	return &AggregateFromDNSRecordsHandler{client: c}
}

func (h *AggregateFromDNSRecordsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNS, ChainData]) error {
	var list v1alpha2.DNSRecordList
	if err := h.client.List(ctx, &list,
		client.InNamespace(rc.Resource.Namespace),
		client.MatchingFields{"spec.portalRef": rc.Resource.Spec.PortalRef},
	); err != nil {
		return err
	}
	rc.Data.DNSRecords = list.Items
	return nil
}
```

Ajouter `DNSRecords []v1alpha2.DNSRecord` à `ChainData` du package `dns/chain`.

- [ ] **Step 2: Écrire le test**

```go
// internal/controller/dns/chain/aggregate_dnsrecords_test.go
package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestAggregateFromDNSRecordsHandler(t *testing.T) {
	g := NewWithT(t)

	dr := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
		Spec:       v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginAuto, PortalRef: "main", SourceType: "ingress"},
	}
	c := fake.NewClientBuilder().WithObjects(dr).Build()

	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       v1alpha2.DNSSpec{PortalRef: "main"},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNS, dnschain.ChainData]{Resource: dns}

	h := dnschain.NewAggregateFromDNSRecordsHandler(c)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(rc.Data.DNSRecords).To(HaveLen(1))
}
```

- [ ] **Step 3: Lancer les tests**

```bash
go test ./internal/controller/dns/chain/... -v
```
Expected: PASS

- [ ] **Step 4: Mettre à jour DNSReconciler**

Modifier `NewDNSReconciler` pour supprimer `disableDNSCheck` et simplifier la chaîne :

```go
func NewDNSReconciler(c client.Client, scheme *runtime.Scheme) *DNSReconciler {
    handlers := []reconciler.Handler[*v1alpha2.DNS, dnschain.ChainData]{
        dnschain.NewAggregateFromDNSRecordsHandler(c),
        dnschain.NewBuildGroupStatusHandler(),   // construit status.groups depuis ChainData.DNSRecords
        dnschain.NewUpdateStatusHandler(c),
    }
    return &DNSReconciler{
        Client: c,
        Scheme: scheme,
        chain:  reconciler.NewChain("dns", handlers...),
    }
}
```

Supprimer `SetFQDNWriter` du `DNSReconciler` (la projection dans le ReadStore se fait uniquement via `DNSRecordReconciler` désormais).

Supprimer les anciens handlers : `CollectManualEntriesHandler`, `AggregateFQDNsHandler`, `ResolveDNSHandler` (du package dns/chain), `ReconcileManualComponentsHandler`.

- [ ] **Step 5: Vérifier compilation**

```bash
go build ./internal/controller/dns/...
```

- [ ] **Step 6: Lancer les tests DNS**

```bash
go test ./internal/controller/dns/... -v
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/controller/dns/
git commit -m "refactor(dns): simplify chain to aggregate-only, remove manual entry handlers"
```

---

## Task 14: Refactor SourceReconciler — per-DNS config

**Files:**
- Modify: `internal/controller/source/source_controller.go`

- [ ] **Step 1: Mettre à jour la struct**

```go
type SourceReconciler struct {
    client.Client
    sourceFactory   *source.Factory
    dynamicClient   dynamic.Interface
    discoveryClient discovery.DiscoveryInterface
    chain           *reconciler.Chain[struct{}, sourcechain.ChainData]

    configsMu sync.RWMutex
    configs   map[string]*ResolvedDNSConfig // key: dns.Name (== portalRef)

    gvrCacheMu sync.RWMutex
    gvrCache   map[schema.GroupResource]schema.GroupVersionResource
    sourceFailures map[registry.SourceType]int

    configChanged chan string // notifié par DNSConfigReconciler
}

type ResolvedDNSConfig struct {
    DNSName        string
    Namespace      string
    Generation     int64
    Reconciliation v1alpha2.ReconciliationSpec
    Sources        v1alpha2.SourcesSpec
    GroupMapping   v1alpha2.GroupMappingSpec
    TypedSources   []registry.TypedSource
    LastTick       time.Time
}
```

- [ ] **Step 2: Mettre à jour le constructeur**

```go
func NewSourceReconciler(
    c client.Client,
    kubeClient kubernetes.Interface,
    restConfig *rest.Config,
    builders []registry.Builder,
) *SourceReconciler {
    // supprimer cfg *config.OperatorConfig et sourcePriority []string
    // ...
    r := &SourceReconciler{
        // ...
        configs:       make(map[string]*ResolvedDNSConfig),
        configChanged: make(chan string, 16),
    }
    return r
}
```

- [ ] **Step 3: Mettre à jour Start()**

```go
func (r *SourceReconciler) Start(ctx context.Context) error {
    logger := log.Default().WithName("source")

    // Charger les configs initiales depuis les DNS CRs
    if err := r.reloadAllDNSConfigs(ctx); err != nil {
        logger.Error(err, "failed to load DNS configs at startup")
    }

    minInterval := r.computeMinInterval()
    ticker := time.NewTicker(minInterval)
    defer ticker.Stop()

    if err := r.reconcileAll(ctx); err != nil {
        logger.Error(err, "initial reconciliation failed")
    }

    for {
        select {
        case <-ctx.Done():
            return nil
        case portalName := <-r.configChanged:
            logger.V(1).Info("DNS config changed, reloading", "portal", portalName)
            if err := r.reloadDNSConfig(ctx, portalName); err != nil {
                logger.Error(err, "failed to reload DNS config", "portal", portalName)
            }
            if err := r.reconcileOne(ctx, portalName); err != nil {
                logger.Error(err, "reconciliation failed after config change", "portal", portalName)
            }
        case <-ticker.C:
            if err := r.reconcileAll(ctx); err != nil {
                logger.Error(err, "periodic reconciliation failed")
            }
            // recalculer l'intervalle si les configs ont changé
            newMin := r.computeMinInterval()
            if newMin != minInterval {
                minInterval = newMin
                ticker.Reset(minInterval)
            }
        }
    }
}
```

- [ ] **Step 4: Implémenter reloadAllDNSConfigs, reloadDNSConfig, reconcileAll, reconcileOne**

```go
func (r *SourceReconciler) reloadAllDNSConfigs(ctx context.Context) error {
    var list v1alpha2.DNSList
    if err := r.Client.List(ctx, &list); err != nil {
        return err
    }
    r.configsMu.Lock()
    defer r.configsMu.Unlock()
    for i := range list.Items {
        dns := &list.Items[i]
        if dns.Spec.IsRemote {
            continue
        }
        r.configs[dns.Name] = &ResolvedDNSConfig{
            DNSName:        dns.Name,
            Namespace:      dns.Namespace,
            Generation:     dns.Generation,
            Sources:        dns.Spec.Sources,
            GroupMapping:   dns.Spec.GroupMapping,
            Reconciliation: dns.Spec.Reconciliation,
        }
    }
    return nil
}

func (r *SourceReconciler) reloadDNSConfig(ctx context.Context, portalName string) error {
    var dns v1alpha2.DNS
    // chercher dans toutes les namespaces (le nom est unique par naming convention)
    var list v1alpha2.DNSList
    if err := r.Client.List(ctx, &list, client.MatchingFields{"spec.portalRef": portalName}); err != nil {
        return err
    }
    for i := range list.Items {
        if list.Items[i].Name == portalName {
            dns = list.Items[i]
            break
        }
    }
    if dns.Name == "" {
        return nil
    }
    r.configsMu.Lock()
    defer r.configsMu.Unlock()
    existing := r.configs[portalName]
    if existing != nil && existing.Generation == dns.Generation {
        return nil // pas de changement
    }
    r.configs[portalName] = &ResolvedDNSConfig{
        DNSName:        dns.Name,
        Namespace:      dns.Namespace,
        Generation:     dns.Generation,
        Sources:        dns.Spec.Sources,
        GroupMapping:   dns.Spec.GroupMapping,
        Reconciliation: dns.Spec.Reconciliation,
    }
    // invalider typedSources pour forcer rebuild
    r.configs[portalName].TypedSources = nil
    return nil
}

func (r *SourceReconciler) computeMinInterval() time.Duration {
    const defaultInterval = 5 * time.Minute
    const minAllowed = 30 * time.Second
    r.configsMu.RLock()
    defer r.configsMu.RUnlock()
    min := defaultInterval
    for _, cfg := range r.configs {
        iv := cfg.Reconciliation.Interval.Duration
        if iv > 0 && iv < min {
            min = iv
        }
    }
    if min < minAllowed {
        return minAllowed
    }
    return min
}

func (r *SourceReconciler) reconcileAll(ctx context.Context) error {
    r.configsMu.RLock()
    names := make([]string, 0, len(r.configs))
    for name := range r.configs {
        names = append(names, name)
    }
    r.configsMu.RUnlock()
    for _, name := range names {
        if err := r.reconcileOne(ctx, name); err != nil {
            log.Default().WithName("source").Error(err, "reconciliation failed", "portal", name)
        }
    }
    return nil
}
```

- [ ] **Step 5: Vérifier compilation**

```bash
go build ./internal/controller/source/...
```

- [ ] **Step 6: Lancer les tests source**

```bash
go test ./internal/controller/source/... -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/controller/source/source_controller.go
git commit -m "refactor(source): per-DNS config map, remove global OperatorConfig dependency"
```

---

## Task 15: DNSConfigNotifier controller

**Files:**
- Create: `internal/controller/source/dns_config_notifier.go`

- [ ] **Step 1: Implémenter**

```go
// internal/controller/source/dns_config_notifier.go
package source

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// DNSConfigNotifier is a minimal controller that watches DNS CRs and notifies
// the SourceReconciler when a DNS spec changes. It does not modify any state.
type DNSConfigNotifier struct {
	client.Client
	notify func(portalName string)
}

func NewDNSConfigNotifier(c client.Client, notify func(string)) *DNSConfigNotifier {
	return &DNSConfigNotifier{Client: c, notify: notify}
}

func (r *DNSConfigNotifier) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.notify(req.Name)
	return ctrl.Result{}, nil
}

func (r *DNSConfigNotifier) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.DNS{}).
		Named("dns-config-notifier").
		Complete(r)
}
```

- [ ] **Step 2: Ajouter la méthode Notify sur SourceReconciler**

```go
// Notify est appelé par DNSConfigNotifier quand une DNS CR change.
func (r *SourceReconciler) Notify(portalName string) {
    select {
    case r.configChanged <- portalName:
    default: // channel plein, le tick régulier prendra en charge
    }
}
```

- [ ] **Step 3: Vérifier compilation**

```bash
go build ./internal/controller/source/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/controller/source/dns_config_notifier.go
git commit -m "feat(source): add DNSConfigNotifier controller for hot-reload"
```

---

## Task 16: Adapter cmd/main.go

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Supprimer la dépendance config pour sources/groupMapping/reconciliation**

Chercher dans `cmd/main.go` les sections :
```go
sourcePriority = source.FilterPriorityOrder(operatorConfig.Sources.Priority, sourceBuilders, operatorConfig)
disableDNSCheck = operatorConfig.Reconciliation.DisableDNSCheck
groupMapping = &operatorConfig.GroupMapping
```
et les supprimer. Ces valeurs sont maintenant lues depuis les DNS CRs par les controllers.

- [ ] **Step 2: Mettre à jour les constructeurs**

```go
// Remplacer:
dnsRecordReconciler := dnsrecordsctrl.NewDNSRecordReconciler(
    mgr.GetClient(), mgr.GetScheme(), groupMapping, dnschain.NewNetResolver(), disableDNSCheck,
)

// Par:
dnsRecordReconciler := dnsrecordsctrl.NewDNSRecordReconciler(
    mgr.GetClient(), mgr.GetScheme(), dnschain.NewNetResolver(),
)

// Remplacer:
dnsReconciler := dnsctrl.NewDNSReconciler(mgr.GetClient(), mgr.GetScheme(), disableDNSCheck)

// Par:
dnsReconciler := dnsctrl.NewDNSReconciler(mgr.GetClient(), mgr.GetScheme())

// Remplacer:
sourceReconciler := sourcectrl.NewSourceReconciler(
    mgr.GetClient(), kubeClient, restCon, operatorConfig, sourceBuilders, sourcePriority,
)

// Par:
sourceReconciler := sourcectrl.NewSourceReconciler(
    mgr.GetClient(), kubeClient, restConfig, sourceBuilders,
)
```

- [ ] **Step 3: Enregistrer DNSConfigNotifier et webhooks v1alpha2**

```go
// Après sourceReconciler.SetupWithManager(mgr):
dnsConfigNotifier := sourcectrl.NewDNSConfigNotifier(mgr.GetClient(), sourceReconciler.Notify)
if err := dnsConfigNotifier.SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "DNSConfigNotifier")
    os.Exit(1)
}

// Dans la section webhooks (ENABLE_WEBHOOKS):
if err := webhookv1alpha2.NewDNSCustomValidator(mgr.GetClient()).SetupWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "DNS/v1alpha2")
    os.Exit(1)
}
controllerSA := os.Getenv("SREPORTAL_CONTROLLER_SA")
if err := webhookv1alpha2.NewDNSRecordCustomValidator(mgr.GetClient(), controllerSA).SetupWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "DNSRecord/v1alpha2")
    os.Exit(1)
}
```

- [ ] **Step 4: Supprimer SetFQDNWriter sur DNSReconciler**

```go
// Supprimer la ligne:
dnsReconciler.SetFQDNWriter(fqdnStore)
```

- [ ] **Step 5: Vérifier compilation complète**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 6: Lancer tous les tests**

```bash
go test -race ./... 2>&1 | tail -30
```

- [ ] **Step 7: Commit**

```bash
git add cmd/main.go
git commit -m "refactor(main): wire v1alpha2 controllers, remove config dependency for DNS"
```

---

## Task 17: Outil de migration hack/migrate-dns-v2

**Files:**
- Create: `hack/migrate-dns-v2/main.go`

- [ ] **Step 1: Implémenter l'outil**

```go
// hack/migrate-dns-v2/main.go
// Usage: go run ./hack/migrate-dns-v2 --kubeconfig <path> [--dry-run]
//
// Pour chaque DNS CR:
//   1. Lit l'annotation sreportal.io/v1alpha1-groups (posée par le webhook de conversion).
//   2. Crée un DNSRecord origin=manual par groupe non-vide.
//   3. Supprime l'annotation après création réussie.
//   4. Patche spec.sources/groupMapping/reconciliation depuis le ConfigMap si existant.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const annotationV1Alpha1Groups = "sreportal.io/v1alpha1-groups"

func main() {
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig")
	dryRun := flag.Bool("dry-run", false, "print actions without applying")
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubeconfig: %v\n", err)
		os.Exit(1)
	}

	scheme := newScheme()
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	var dnsList v1alpha2.DNSList
	if err := c.List(ctx, &dnsList); err != nil {
		fmt.Fprintf(os.Stderr, "list DNS: %v\n", err)
		os.Exit(1)
	}

	for i := range dnsList.Items {
		dns := &dnsList.Items[i]
		raw, ok := dns.Annotations[annotationV1Alpha1Groups]
		if !ok || raw == "" {
			fmt.Printf("DNS %s/%s: no v1alpha1 groups annotation, skipping\n", dns.Namespace, dns.Name)
			continue
		}

		var groups []v1alpha1.DNSGroup
		if err := json.Unmarshal([]byte(raw), &groups); err != nil {
			fmt.Fprintf(os.Stderr, "DNS %s/%s: parse groups: %v\n", dns.Namespace, dns.Name, err)
			continue
		}

		for _, g := range groups {
			if len(g.Entries) == 0 {
				continue
			}
			recordName := dns.Name + "-manual-" + slug(g.Name)
			entries := make([]v1alpha2.DNSRecordEntry, 0, len(g.Entries))
			for _, e := range g.Entries {
				entries = append(entries, v1alpha2.DNSRecordEntry{
					FQDN:        e.FQDN,
					Group:       g.Name,
					Description: e.Description,
					RecordType:  "A",
				})
			}
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recordName,
					Namespace: dns.Namespace,
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:    v1alpha2.DNSRecordOriginManual,
					PortalRef: dns.Spec.PortalRef,
					Entries:   entries,
				},
			}
			if *dryRun {
				fmt.Printf("[dry-run] would create DNSRecord %s/%s (%d entries)\n", dns.Namespace, recordName, len(entries))
				continue
			}
			if err := c.Create(ctx, record); err != nil {
				fmt.Fprintf(os.Stderr, "create DNSRecord %s/%s: %v\n", dns.Namespace, recordName, err)
				continue
			}
			fmt.Printf("created DNSRecord %s/%s\n", dns.Namespace, recordName)
		}

		if !*dryRun {
			patch := client.MergeFrom(dns.DeepCopy())
			delete(dns.Annotations, annotationV1Alpha1Groups)
			if err := c.Patch(ctx, dns, patch); err != nil {
				fmt.Fprintf(os.Stderr, "remove annotation from DNS %s/%s: %v\n", dns.Namespace, dns.Name, err)
			}
		}
	}
	fmt.Println("migration complete")
}

func slug(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result = append(result, byte(c))
		} else if c >= 'A' && c <= 'Z' {
			result = append(result, byte(c+32))
		} else {
			result = append(result, '-')
		}
	}
	return string(result)
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	_ = v1alpha2.AddToScheme(s)
	return s
}
```

- [ ] **Step 2: Vérifier compilation**

```bash
go build ./hack/migrate-dns-v2/...
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add hack/migrate-dns-v2/
git commit -m "feat(hack): add migrate-dns-v2 migration tool"
```

---

## Task 18: Samples v1alpha2 + lint + tests finaux

**Files:**
- Create: `config/samples/sreportal_v1alpha2_dns.yaml`
- Create: `config/samples/sreportal_v1alpha2_dnsrecord_manual.yaml`

- [ ] **Step 1: Créer les samples**

```yaml
# config/samples/sreportal_v1alpha2_dns.yaml
apiVersion: sreportal.io/v1alpha2
kind: DNS
metadata:
  name: main
  namespace: default
spec:
  portalRef: main
  sources:
    ingress:
      enabled: true
      annotationFilter: "external-dns.alpha.kubernetes.io/hostname"
    service:
      enabled: true
      serviceTypeFilter: [LoadBalancer]
    priority: [ingress, service]
  groupMapping:
    defaultGroup: Services
    labelKey: sreportal.io/group
    byNamespace:
      monitoring: Monitoring
      default: Development
  reconciliation:
    interval: 5m
    retryOnError: 30s
    disableDNSCheck: false
```

```yaml
# config/samples/sreportal_v1alpha2_dnsrecord_manual.yaml
apiVersion: sreportal.io/v1alpha2
kind: DNSRecord
metadata:
  name: main-manual-apis
  namespace: default
spec:
  origin: manual
  portalRef: main
  entries:
    - fqdn: api.example.com
      group: APIs
      description: Main API endpoint
      recordType: A
    - fqdn: graphql.example.com
      group: APIs
      description: GraphQL API
```

- [ ] **Step 2: Lancer make helm + make doc**

```bash
make helm
make doc
```

- [ ] **Step 3: Lancer lint**

```bash
make lint
```
Corriger les éventuels problèmes signalés.

- [ ] **Step 4: Lancer la suite de tests complète**

```bash
go test -race -cover ./... 2>&1 | tail -50
```
Expected: PASS, coverage ≥ 80% sur les packages modifiés

- [ ] **Step 5: Commit final**

```bash
git add config/samples/sreportal_v1alpha2_dns.yaml config/samples/sreportal_v1alpha2_dnsrecord_manual.yaml
git commit -m "feat(samples): add v1alpha2 DNS and DNSRecord manual samples"
```

---

## Auto-review du plan

**Couverture spec :**
- ✅ §1bis — versioning v1alpha2 (Tasks 1–3)
- ✅ §2 — DNS CRD nouveau schéma (Task 2)
- ✅ §3 — DNSRecord dual-purpose (Task 3)
- ✅ §4.1 — SourceReconciler per-DNS (Task 14)
- ✅ §4.2 — DNSRecordReconciler adaptation (Tasks 9–12)
- ✅ §4.3 — DNSReconciler simplifié (Task 13)
- ✅ §5 — Migration tool (Task 17)
- ✅ §6bis — Conversion webhook (Tasks 4–5)
- ✅ §6.3 — Webhooks v1alpha2 (Tasks 6–7)
- ✅ §7 — Plan implémentation (cet ordre)
- ✅ §8 — Samples (Task 18)

**Cohérence des types :**
- `v1alpha2.GroupMappingSpec` utilisé partout de façon cohérente (Tasks 11, 12, 14)
- `v1alpha2.DNSRecordOriginAuto/Manual` défini en Task 2, utilisé Tasks 3, 5, 7, 10, 11
- `ChainData.GroupMapping *v1alpha2.GroupMappingSpec` défini Task 9, lu Tasks 11, 12
- `NewProjectStoreHandler(w)` — suppression du paramètre groupMapping en Task 11, cohérent avec Task 12
- `NewDNSRecordReconciler(c, scheme, resolver)` — nouveau constructeur Task 12, mis à jour Task 16
- `NewDNSReconciler(c, scheme)` — `disableDNSCheck` supprimé Task 13, cohérent Task 16
- `NewSourceReconciler(c, kubeClient, restConfig, builders)` — Task 14, cohérent Task 16
