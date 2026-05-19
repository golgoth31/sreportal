# Spec — Refonte de la couche DNS : distribution de la configuration via la CRD DNS

Date : 2026-05-14
Statut : Design — prêt pour implémentation
Auteur : Architecture Go/Kubernetes
Périmètre : DNS, DNSRecord, SourceReconciler, DNSRecordReconciler, suppression de la CRD DNS legacy (groups inline)
Versions CRD : `DNS` v1alpha1 → v1alpha2 / `DNSRecord` v1alpha1 → v1alpha2 (conversion webhook)

---

## 1. Contexte et objectifs

### 1.1 État actuel

- `OperatorConfig` (ConfigMap) porte `sources`, `groupMapping`, `reconciliation` — partagé par tous les portals/DNS.
- La CRD `DNS` :
  - `spec.portalRef` (lien vers Portal)
  - `spec.groups[].entries[]` (entrées manuelles inline, DNSEntry)
  - `spec.isRemote` (flag pour DNS remote géré par portal controller)
  - `status.groups` (agrégation des FQDN découverts et manuels)
- La CRD `DNSRecord` :
  - Créée automatiquement par `SourceReconciler` (un par couple `portal × sourceType`)
  - Nommée `<portal>-<sourceType>`
  - `spec.sourceType` + `spec.portalRef`
  - `status.endpoints` rempli par le source controller
- `DNSRecordReconciler` projette `DNSRecord.status.endpoints` vers le ReadStore FQDN.
- `DNSReconciler` collecte les `spec.groups[].entries[]` manuels et les projette dans le ReadStore.

### 1.2 Objectifs de la refonte

1. **Décentraliser la configuration** : chaque DNS CR porte sa propre `SourcesConfig`, `GroupMappingConfig`, `ReconciliationConfig`. Plus de ConfigMap unique.
2. **Relation 1:1 stricte Portal ↔ DNS** : un Portal a exactement une DNS CR.
3. **Supprimer `DNSEntry` de `DNS.spec`** : les entrées manuelles deviennent des `DNSRecord` à part entière (dual purpose).
4. **`DNSRecord` dual-purpose** : auto-discovered (source controller) **ou** manuel (utilisateur).
5. Conservation du modèle Chain-of-Responsibility et du ReadStore CQRS.

### 1.3 Décisions actées (non négociables)

- 1 Portal = 1 DNS CR (enforcée par webhook + naming convention).
- DNS CR porte `spec.sources`, `spec.groupMapping`, `spec.reconciliation`.
- `DNSEntry` et `spec.groups[]` disparaissent de la CRD DNS.
- `DNSRecord` accepte manuel + auto.
- Lien DNS ↔ DNSRecord via `spec.portalRef` (inchangé).
- `DNS` et `DNSRecord` passent en **v1alpha2** ; v1alpha1 reste served pour la compatibilité le temps de la migration.
- Un **conversion webhook** gère la transformation v1alpha1 ↔ v1alpha2.
- Auth et Release : hors périmètre.

---

## 1bis. Stratégie de versioning CRD

### 1bis.1 Pourquoi v1alpha2

Les changements apportés à `DNS` (suppression de `spec.groups[]`, ajout de `spec.sources/groupMapping/reconciliation`) et à `DNSRecord` (ajout de `spec.origin`) sont **breaking** : un objet v1alpha1 stocké ne peut pas être lu tel quel par un controller v1alpha2. Kubernetes impose un conversion webhook dès qu'une CRD expose plusieurs versions avec des schémas incompatibles.

### 1bis.2 Versions served / stored

| CRD | Version stored | Versions served | Remarque |
|---|---|---|---|
| `DNS` | `v1alpha2` | `v1alpha1`, `v1alpha2` | v1alpha1 served pour rollback et migration outillée |
| `DNSRecord` | `v1alpha2` | `v1alpha1`, `v1alpha2` | idem |

Markers kubebuilder :
```go
// api/v1alpha2/dns_types.go
// +kubebuilder:storageversion

// api/v1alpha1/dns_types.go
// (pas de +kubebuilder:storageversion — v1alpha1 reste served uniquement)
```

### 1bis.3 Hub version

Le pattern Kubebuilder recommandé pour la conversion multi-version est le **hub spoke** : une version est déclarée "hub" (la version de stockage, ici `v1alpha2`). Les autres versions implémentent `ConvertTo(hub)` et `ConvertFrom(hub)`.

```go
// api/v1alpha2/dns_types.go
func (*DNS) Hub() {} // marqueur interface conversion.Hub

// api/v1alpha1/dns_types.go
func (src *DNS) ConvertTo(dstRaw conversion.Hub) error { ... }
func (dst *DNS) ConvertFrom(srcRaw conversion.Hub) error { ... }
```

### 1bis.4 Nouveau package `api/v1alpha2`

Les types v1alpha2 sont dans un nouveau package `api/v1alpha2/` (convention Kubebuilder). La commande :

```bash
kubebuilder create api --group sreportal --version v1alpha2 --kind DNS --resource --controller=false
kubebuilder create api --group sreportal --version v1alpha2 --kind DNSRecord --resource --controller=false
```

génère le squelette. Les controllers (`source_controller`, `dnsrecord_controller`, `dns_controller`) sont mis à jour pour importer `v1alpha2` (la version de stockage).

---

## 2. Nouveau schéma de la CRD `DNS`

### 2.1 Champs supprimés

- `spec.groups[]` (avec `DNSEntry`, `DNSGroup`) — migré vers `DNSRecord` manuels.

### 2.2 Champs conservés

- `spec.portalRef` — clé d'unicité (1:1).
- `spec.isRemote` — comportement inchangé (skip reconcile pour DNS gérés par portal controller distant).
- `status.groups`, `status.conditions`, `status.lastReconcileTime` — inchangés (status agrégé pour le UI).

### 2.3 Champs ajoutés

```go
// api/v1alpha1/dns_types.go

// DNSSpec defines the desired state of DNS.
// One Portal maps to exactly one DNS CR (enforced by webhook + naming convention).
type DNSSpec struct {
    // portalRef is the name of the Portal this DNS resource is linked to.
    // The DNS CR name MUST equal portalRef (enforced by webhook).
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    PortalRef string `json:"portalRef"`

    // isRemote indicates this DNS resource is managed by the portal controller
    // for a remote portal. When true, the DNS controller skips reconciliation.
    // +optional
    IsRemote bool `json:"isRemote,omitempty"`

    // sources enables and configures each external-dns source type for this DNS.
    // Replaces the cluster-wide ConfigMap.sources. Empty means no source is active.
    // +optional
    Sources SourcesSpec `json:"sources,omitempty"`

    // groupMapping configures how FQDNs are organised into groups in the UI.
    // Replaces ConfigMap.groupMapping.
    // +kubebuilder:default={defaultGroup:"Services"}
    // +optional
    GroupMapping GroupMappingSpec `json:"groupMapping,omitempty"`

    // reconciliation controls timing of the source poll loop for this DNS.
    // Replaces ConfigMap.reconciliation.
    // +kubebuilder:default={interval:"5m",retryOnError:"30s"}
    // +optional
    Reconciliation ReconciliationSpec `json:"reconciliation,omitempty"`
}
```

### 2.4 Sous-structures (clone de `internal/config` en types Kubebuilder)

`internal/config.SourcesConfig`, `GroupMappingConfig`, `ReconciliationConfig` deviennent des types CRD (`SourcesSpec`, `GroupMappingSpec`, `ReconciliationSpec`) définis directement dans `api/v1alpha1/dns_types.go` (pas de fichier séparé — cohérent avec les autres CRDs du projet qui ont tous leurs sous-types inline). Le champ `Duration` (string -> time.Duration parsé en Go) doit être typé `metav1.Duration` car ce dernier supporte nativement JSON-Schema + marshaling Kubernetes.

```go
// SourcesSpec mirrors internal/config.SourcesConfig but with Kubebuilder markers.
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

    // priority defines source precedence on FQDN+RecordType collision.
    // +kubebuilder:validation:items:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
    // +optional
    Priority []string `json:"priority,omitempty"`
}

// ServiceSourceSpec, IngressSourceSpec, etc. dupliquent les *Config existantes,
// champ-pour-champ, sans changement sémantique. Les noms YAML sont identiques
// pour faciliter la migration depuis le ConfigMap.

type GroupMappingSpec struct {
    // +kubebuilder:default="Services"
    // +kubebuilder:validation:MinLength=1
    DefaultGroup string `json:"defaultGroup"`
    // +optional
    LabelKey string `json:"labelKey,omitempty"`
    // +optional
    ByNamespace map[string]string `json:"byNamespace,omitempty"`
}

type ReconciliationSpec struct {
    // +kubebuilder:default="5m"
    Interval metav1.Duration `json:"interval"`
    // +kubebuilder:default="30s"
    RetryOnError metav1.Duration `json:"retryOnError"`
    // +optional
    DisableDNSCheck bool `json:"disableDNSCheck,omitempty"`
}
```

### 2.5 Status — additions

```go
type DNSStatus struct {
    // groups, conditions, lastReconcileTime — inchangés.
    Groups            []FQDNGroupStatus  `json:"groups,omitempty"`
    Conditions        []metav1.Condition `json:"conditions,omitempty"`
    LastReconcileTime *metav1.Time       `json:"lastReconcileTime,omitempty"`

    // observedGeneration is the .metadata.generation reflected in this status.
    // Used to detect that the source loop has reloaded the new spec.
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // activeSources lists the source types currently active in the loop.
    // Surface visible pour debug + UI ("4 sources actives").
    // +optional
    ActiveSources []string `json:"activeSources,omitempty"`

    // nextReconcileTime is the projected timestamp of the next source tick.
    // +optional
    NextReconcileTime *metav1.Time `json:"nextReconcileTime,omitempty"`
}
```

### 2.6 Relation 1:1 — comment l'enforcer

**Convention de nommage + webhook** (suffisant — pas de field indexer custom requis) :

1. **Naming convention** : `DNS.metadata.name == DNS.spec.portalRef`. Garantit l'unicité Portal↔DNS via la contrainte naturelle de noms d'objets Kubernetes (unique par namespace).
2. **Webhook ValidateCreate** (extension de `dns_webhook.go`) :
   - Vérifier `obj.Name == obj.Spec.PortalRef`. Reject sinon.
   - Vérifier que le Portal référencé existe (déjà en place).
3. **Webhook ValidateUpdate** :
   - `spec.portalRef` immuable. Reject toute modification.
4. **Webhook côté Portal** (optionnel mais recommandé) : à la suppression d'un Portal, refuser si une DNS CR le référence (finalizer), ou laisser le garbage collector via `ownerReference` (voir 2.7).

### 2.7 OwnerReference (recommandé)

Lors de la création d'une DNS CR, le webhook **defaulter** (`DNSCustomDefaulter.Default`) ajoute un `metav1.OwnerReference` pointant vers le Portal référencé (BlockOwnerDeletion = false, Controller = false). Avantage : suppression automatique de la DNS CR si le Portal disparaît. Le Portal reste « propriétaire logique ».

---

## 3. Nouveau schéma de la CRD `DNSRecord`

### 3.1 Discriminer manuel vs auto-découvert

Ajout d'un champ `spec.origin` (enum) :

```go
// +kubebuilder:validation:Enum=auto;manual
type DNSRecordOrigin string

const (
    DNSRecordOriginAuto   DNSRecordOrigin = "auto"
    DNSRecordOriginManual DNSRecordOrigin = "manual"
)
```

Règles :
- `origin=auto` : créé/géré par `SourceReconciler`. `spec.sourceType` requis. `spec.entries` interdit (vide).
- `origin=manual` : créé par l'utilisateur. `spec.sourceType` interdit. `spec.entries` requis (≥1).

### 3.2 Schéma complet proposé

```go
// DNSRecordSpec defines the desired state of DNSRecord.
// +kubebuilder:validation:XValidation:rule="self.origin == 'auto' ? has(self.sourceType) && (!has(self.entries) || size(self.entries) == 0) : !has(self.sourceType) && has(self.entries) && size(self.entries) > 0",message="auto records require sourceType and no entries; manual records require entries and no sourceType"
type DNSRecordSpec struct {
    // origin distinguishes auto-discovered (by SourceReconciler) records from
    // user-managed manual records.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=auto;manual
    // +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.origin is immutable"
    Origin DNSRecordOrigin `json:"origin"`

    // portalRef is the name of the Portal this record belongs to.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.portalRef is immutable"
    PortalRef string `json:"portalRef"`

    // sourceType is the external-dns source type that produced this record.
    // Required when origin=auto. Must be empty when origin=manual.
    // +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
    // +optional
    SourceType string `json:"sourceType,omitempty"`

    // entries is the list of manual DNS entries. Required when origin=manual.
    // +optional
    // +listType=map
    // +listMapKey=fqdn
    Entries []DNSRecordEntry `json:"entries,omitempty"`
}

// DNSRecordEntry is a single manual DNS entry inside a DNSRecord of origin=manual.
type DNSRecordEntry struct {
    // fqdn is the fully qualified domain name. Required.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:Pattern=`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
    FQDN string `json:"fqdn"`

    // group is the UI group this entry belongs to. When empty,
    // groupMapping.defaultGroup is used.
    // +optional
    Group string `json:"group,omitempty"`

    // description is an optional human-readable description.
    // +optional
    Description string `json:"description,omitempty"`

    // recordType is the DNS record type (A, AAAA, CNAME, TXT). Default "A".
    // +kubebuilder:validation:Enum=A;AAAA;CNAME;TXT
    // +kubebuilder:default="A"
    // +optional
    RecordType string `json:"recordType,omitempty"`

    // targets is the list of target addresses (IPs or hostnames).
    // Optional: when empty, the record is treated as "registered" but the
    // ResolveDNSHandler will perform live DNS lookup to fill SyncStatus.
    // +optional
    Targets []string `json:"targets,omitempty"`
}
```

### 3.3 Status — adapté

`status.endpoints` reste valable pour les deux origines. Pour `origin=manual`, le DNSRecord controller remplit `status.endpoints` à partir de `spec.entries` (avec résolution DNS optionnelle pour `syncStatus`), normalisant la projection vers le ReadStore.

```go
type DNSRecordStatus struct {
    // endpoints contains the DNS endpoints for this record.
    // For origin=auto: filled by SourceReconciler.
    // For origin=manual: filled by DNSRecordReconciler from spec.entries.
    Endpoints []EndpointStatus `json:"endpoints,omitempty"`

    // endpointsHash — auto only (used by SourceReconciler to skip updates).
    EndpointsHash string `json:"endpointsHash,omitempty"`

    LastReconcileTime *metav1.Time       `json:"lastReconcileTime,omitempty"`
    Conditions        []metav1.Condition `json:"conditions,omitempty"`

    // observedGeneration is the .metadata.generation reflected in this status.
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
```

### 3.4 Naming convention

- `origin=auto` : `<portalRef>-<sourceType>` (inchangé).
- `origin=manual` : libre (l'utilisateur choisit). Convention recommandée : `<portalRef>-manual-<scope>` (ex : `main-manual-apis`).

Aucune contrainte technique sur le nom manuel ; seul `portalRef` détermine le rattachement.

---

## 4. Impact sur les controllers

### 4.1 `SourceReconciler` — lecture config depuis les DNS CR

Aujourd'hui, le `SourceReconciler` est instancié dans `cmd/main.go` avec **une** `*config.OperatorConfig` globale et un ticker unique. Avec la refonte, **chaque DNS CR porte sa propre config**.

**Décision pragmatique** : conserver un seul `SourceReconciler` (Runnable du manager), mais transformer son state interne en map `portalName -> resolvedConfig` :

```go
type SourceReconciler struct {
    client.Client
    // ...
    configsMu sync.RWMutex
    configs   map[string]*ResolvedDNSConfig // key: portalRef (== dns.Name)

    // Un ticker unique réveille la boucle ; chaque tick scanne toutes les
    // DNS CR (cache controller-runtime) et exécute la chaîne par DNS.
    minInterval time.Duration

    // configChanged est notifié par DNSConfigReconciler à chaque update DNS CR.
    configChanged chan string
}

type ResolvedDNSConfig struct {
    DNSName        string // == portalRef
    Namespace      string
    Generation     int64
    Sources        v1alpha1.SourcesSpec
    GroupMapping   v1alpha1.GroupMappingSpec
    Reconciliation v1alpha1.ReconciliationSpec
    TypedSources   []registry.TypedSource
    LastTick       time.Time
}
```

**Algorithme du tick** :

1. Lister toutes les DNS CR (cache controller-runtime) — `r.List(ctx, &dnsList)`.
2. Pour chaque DNS (sauf `isRemote`) :
   - Si `dns.Generation != cached.Generation` : reconstruire les `typedSources` pour ce DNS (`sourceFactory.BuildTypedSources(ctx, resolved)`).
   - Si `time.Since(cached.LastTick) >= resolved.Reconciliation.Interval.Duration()` : exécuter la chaîne (sub-chain par DNS).
3. La sous-chaîne devient :
   - `RebuildSourcesHandler` (per-DNS, idempotent, gardé par generation)
   - `BuildPortalIndexHandler` → restreint au portal `dns.Spec.PortalRef`
   - `CollectEndpointsHandler` → endpoints filtrés par ce portal seul
   - `DeduplicateHandler` (priority depuis `resolved.Sources.Priority`)
   - `ReconcileDNSRecordsHandler` → crée/maj DNSRecord `origin=auto` (filter sur ce portal)
   - `ReconcileComponentsHandler`
   - `DeleteOrphanedHandler`

**Pas de watch dynamique** : controller-runtime v0.23 ne supporte pas les watches dynamiques pour des informers configurés à la volée. La solution :
- Le ticker scanne périodiquement les DNS CR (cache local, coût marginal).
- Ajouter en parallèle un **controller standard** (`DNSConfigReconciler`) qui réagit aux events DNS et notifie le `SourceReconciler` via un channel (`r.configChanged <- portalName`) → invalide le cache, déclenche un tick anticipé pour ce DNS.

```go
// Notify-on-update controller, minimal — pas de chain, juste un signal.
type DNSConfigReconciler struct {
    client.Client
    Notify func(portalName string)
}

func (r *DNSConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    r.Notify(req.Name) // == portalRef
    return ctrl.Result{}, nil
}

func (r *DNSConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&sreportalv1alpha1.DNS{}).
        Named("dns-config-notifier").
        Complete(r)
}
```

Le `SourceReconciler.Start` écoute `r.configChanged` et déclenche un tick ciblé hors du cycle régulier.

**Tick interval global** : `minInterval = min(dns.spec.reconciliation.interval)` borné à `30s` minimum. Cette valeur dimensionne le ticker maître ; chaque DNS est traité à sa propre cadence via `LastTick`.

### 4.2 `DNSRecordReconciler` — adaptation

Modifications :

1. **Lookup config DNS** : pour récupérer `groupMapping` et `disableDNSCheck`, le reconciler doit charger la DNS CR du portal référencé (cache controller-runtime, pas de coût supplémentaire).
2. **Handle `origin=manual`** : nouveau handler `MaterialiseManualEntriesHandler` exécuté avant `SyncEndpointsHashHandler` :
   - Si `spec.origin == "manual"` : convertir `spec.entries[]` en `status.endpoints` (`lastSeen = now`, `recordType` par défaut "A", labels `sreportal.io/group=<entry.Group>`).
   - Si `origin=auto` : no-op (`status.endpoints` est rempli par le source controller).
3. **`ProjectStoreHandler`** : aucune modification structurelle. Il consomme `record.Status.Endpoints` + `groupMapping` (récupéré depuis la DNS CR). Pour `origin=manual`, `Source` dans la `FQDNView` doit être `domaindns.SourceManual` (pas `SourceExternalDNS`).
4. **Watch sur DNS** : ajout d'un watch `DNS{}` qui re-enqueue tous les DNSRecord du portal en cas de changement de `groupMapping` ou de feature toggle.

```go
func (r *DNSRecordReconciler) rebuildChain() {
    r.chain = reconciler.NewChain(
        "dnsrecord",
        dnsrecordchain.NewLoadDNSConfigHandler(r.Client),         // nouveau : charge DNS CR (groupMapping, disableDNSCheck)
        dnsrecordchain.NewMaterialiseManualEntriesHandler(),      // nouveau : entries -> endpoints
        dnsrecordchain.NewSyncEndpointsHashHandler(r.Client),     // inchangé
        dnsrecordchain.NewResolveDNSHandler(r.Client, r.resolver),// disableDNSCheck lu depuis ChainData
        dnsrecordchain.NewProjectStoreHandler(r.fqdnWriter),      // groupMapping lu depuis ChainData
    )
}
```

`ChainData` ajoute :

```go
type ChainData struct {
    ResourceKey      string
    DNSGroupMapping  *v1alpha1.GroupMappingSpec
    DisableDNSCheck  bool
}
```

### 4.3 `DNSReconciler` — quasi suppression

Le `DNSReconciler` actuel orchestre :
- collecte des `spec.groups[].entries[]` (disparu)
- agrégation FQDNs, résolution, update status
- projection ReadStore

**Avec la refonte** :
- Les entrées manuelles sont gérées par `DNSRecordReconciler` (DNSRecord `origin=manual`).
- Le `DNSReconciler` ne fait plus que :
  - Maintenir `status.groups` (agrégation cosmétique pour le UI legacy), construite en listant les DNSRecord du portal et en délégant à un nouveau handler `AggregateFromDNSRecordsHandler`.
  - Mettre à jour `status.activeSources`, `status.nextReconcileTime`, `status.observedGeneration`.
  - Gérer `isRemote` (skip — inchangé).
  - Ne plus projeter dans le ReadStore : projection 100 % via `DNSRecordReconciler`.

Chaîne simplifiée :

```go
handlers := []reconciler.Handler[*sreportalv1alpha1.DNS, dnschain.ChainData]{
    dnschain.NewListDNSRecordsHandler(c),       // remplit ChainData.Records
    dnschain.NewAggregateStatusHandler(),       // status.groups depuis les DNSRecord
    dnschain.NewUpdateStatusHandler(c),
}
```

Suppression des handlers : `CollectManualEntriesHandler`, `AggregateFQDNsHandler`, `ResolveDNSHandler`, `ReconcileManualComponentsHandler` (déplacé/refactor dans `DNSRecordReconciler`).

### 4.4 Plus de ConfigMap operator (pour la couche DNS)

Le chargement de `OperatorConfig` depuis le ConfigMap reste partiel : `Release`, `Auth`, `Emoji` continuent à venir du ConfigMap (hors périmètre). Mais `Sources`, `GroupMapping`, `Reconciliation` ne sont plus lus.

`cmd/main.go` :
- Conserve le load du ConfigMap (pour Auth/Release/Emoji).
- Le `SourceReconciler` n'est plus instancié avec un `*config.OperatorConfig` global pour les 3 sections retirées : il reçoit uniquement le `client.Client` + `builders`. La `sourcePriority` globale est supprimée (priority désormais par-DNS).

---

## 5. Migration

### 5.1 Pré-conditions

- Aucune compatibilité descendante (préversion `v1alpha1`, breaking change accepté).
- Champs supprimés détectés par le serveur API : update échoue si l'objet a `spec.groups` (rejected by schema). Donc migration **avant** déploiement de la nouvelle CRD.

### 5.2 Ordre des opérations (atomique au déploiement)

1. **Outil de migration** (`hack/migrate-dns-v2/main.go`) :
   - Liste toutes les DNS CR.
   - Pour chaque DNS, pour chaque `group` dans `spec.groups`, pour chaque `entry` :
     - Crée un `DNSRecord` `origin=manual` :
       - `metadata.name = <dnsName>-manual-<slug(group.name)>` (un DNSRecord par groupe).
       - `spec.portalRef = dns.spec.portalRef`.
       - `spec.entries[] = { fqdn, description, group: group.name }`.
   - Lit le ConfigMap operator, et pour chaque DNS CR (sans `isRemote`) :
     - Patche `spec.sources`, `spec.groupMapping`, `spec.reconciliation` avec le contenu du ConfigMap.
   - Patche `spec.groups = null` (pour préparer la suppression).
   - Renomme la DNS CR si `metadata.name != spec.portalRef` (création nouvelle + suppression ancienne, avec ownerReference temporaire).
2. **Déploiement de la nouvelle CRD** (`make manifests` + apply).
3. **Déploiement du nouveau binaire** (helm upgrade).
4. **Vérification** :
   - `kubectl get dnsrecord -A -o jsonpath='{range .items[?(@.spec.origin=="manual")]}{.metadata.name}{"\n"}{end}'` (DNSRecord manuels)
   - `kubectl get dns -A -o jsonpath` (vérifier `spec.sources` populé)
5. **Cleanup ConfigMap** : retirer les sections `sources`, `groupMapping`, `reconciliation`.

### 5.3 Outil de migration — squelette

```go
// hack/migrate-dns-v2/main.go
// Usage: migrate-dns-v2 --kubeconfig <path> --configmap <ns>/<name> [--dry-run]
//
// 1. Load OperatorConfig from ConfigMap.
// 2. List DNS CRs (old schema, accessed via unstructured to bypass typed schema).
// 3. For each DNS:
//    - Create one DNSRecord per group with origin=manual.
//    - Patch DNS with sources/groupMapping/reconciliation from OperatorConfig.
//    - Clear spec.groups.
//    - Rename if metadata.name != spec.portalRef.
// 4. Report.
```

Lecture en `unstructured.Unstructured` indispensable car le binaire de migration doit comprendre **l'ancien** schéma alors que la nouvelle CRD est sur le point d'être déployée.

### 5.4 Rollback

- Sauvegarde préalable de toutes les DNS CR avec `kubectl get dns -A -o yaml > dns-backup-v1.yaml` avant migration.
- Procédure de rollback : redéployer ancienne CRD + ancien binaire + `kubectl apply -f dns-backup-v1.yaml`.

---

## 6. Validation & Defaults

### 6.1 Markers Kubebuilder (defaults sur la CRD)

| Champ | Default | Mécanisme |
|---|---|---|
| `dns.spec.groupMapping.defaultGroup` | `"Services"` | `+kubebuilder:default="Services"` |
| `dns.spec.reconciliation.interval` | `5m` | `+kubebuilder:default="5m"` |
| `dns.spec.reconciliation.retryOnError` | `30s` | `+kubebuilder:default="30s"` |
| `dnsrecord.spec.entries[].recordType` | `"A"` | `+kubebuilder:default="A"` |

### 6.2 Validation CEL (sans webhook)

Sur `DNSRecord` :
```go
// +kubebuilder:validation:XValidation:rule="self.origin == 'auto' ? has(self.sourceType) && (!has(self.entries) || size(self.entries) == 0) : !has(self.sourceType) && has(self.entries) && size(self.entries) > 0",message="auto records require sourceType; manual records require entries"
```

Sur `DNSRecord.spec.entries[].fqdn` : pattern regex DNS standard (cf. §3.2).

Sur `DNSRecord.spec.origin` et `DNSRecord.spec.portalRef` : immutables via CEL `self == oldSelf`.

### 6.3 Validation Webhook (logique nécessitant un lookup)

Sur `DNS` (extension du webhook existant `dns_webhook.go`) :
1. **`Name == Spec.PortalRef`** : reject create/update si mismatch.
2. **`Spec.PortalRef` immuable** : reject update si modifié.
3. **Portal existe** : (déjà en place).
4. **1:1 strict** : la naming convention (`Name == PortalRef`) suffit puisque `Name` est unique par namespace. Pas besoin de field indexer.

Sur `DNSRecord` (nouveau webhook `dnsrecord_webhook.go`) :
1. **Portal existe** : lookup Portal référencé.
2. **DNS CR existe pour le portal** : lookup DNS `name=spec.portalRef`. Reject si absent (orpheline).
3. **`origin=auto` réservé au controller** : `spec.origin == "auto"` rejeté si l'admission ne provient pas d'un ServiceAccount opérateur (vérifier via `admissionRequest.UserInfo.Username`, ex: `system:serviceaccount:<ns>:sreportal-controller`). Externaliser le nom via env var `SREPORTAL_CONTROLLER_SA`.

### 6.4 Récapitulatif CEL vs Webhook

| Règle | Lieu |
|---|---|
| Defaults statiques | Kubebuilder markers |
| Enum, MinLength, Pattern | Kubebuilder markers |
| `origin` ↔ `sourceType`/`entries` cohérence | CEL XValidation |
| Immutabilité (`origin`, `portalRef`) | CEL XValidation |
| Cross-resource lookup (Portal, DNS) | Webhook |
| Naming `DNS.Name == DNS.Spec.PortalRef` | Webhook |
| Auth `origin=auto` réservé au controller SA | Webhook |

---

## 6bis. Conversion webhook v1alpha1 ↔ v1alpha2

### 6bis.1 DNS — règles de conversion

**v1alpha1 → v1alpha2 (`ConvertTo`)**

| Champ v1alpha1 | Champ v1alpha2 | Traitement |
|---|---|---|
| `spec.portalRef` | `spec.portalRef` | copie directe |
| `spec.isRemote` | `spec.isRemote` | copie directe |
| `spec.groups[]` | — | Sérialisé en JSON dans l'annotation `sreportal.io/v1alpha1-groups` de l'objet v1alpha2. Permet à l'outil de migration de créer les DNSRecord manuels correspondants. |
| — | `spec.sources` | Vide (l'outil de migration le remplit depuis le ConfigMap) |
| — | `spec.groupMapping` | Default Kubebuilder (`defaultGroup: "Services"`) |
| — | `spec.reconciliation` | Default Kubebuilder (`interval: 5m`, `retryOnError: 30s`) |

**v1alpha2 → v1alpha1 (`ConvertFrom`) — rollback**

| Champ v1alpha2 | Champ v1alpha1 | Traitement |
|---|---|---|
| `spec.portalRef` | `spec.portalRef` | copie directe |
| `spec.isRemote` | `spec.isRemote` | copie directe |
| annotation `sreportal.io/v1alpha1-groups` | `spec.groups[]` | Désérialisation JSON depuis l'annotation (si présente), sinon `nil` |
| `spec.sources/groupMapping/reconciliation` | — | Perdus (pas de champ équivalent en v1alpha1) |

### 6bis.2 DNSRecord — règles de conversion

**v1alpha1 → v1alpha2 (`ConvertTo`)**

| Champ v1alpha1 | Champ v1alpha2 | Traitement |
|---|---|---|
| `spec.portalRef` | `spec.portalRef` | copie directe |
| `spec.sourceType` | `spec.sourceType` | copie directe |
| — | `spec.origin` | `"auto"` (tous les DNSRecord v1alpha1 existants sont auto-découverts) |
| `status.endpoints` | `status.endpoints` | copie directe |

**v1alpha2 → v1alpha1 (`ConvertFrom`)**

| Champ v1alpha2 | Champ v1alpha1 | Traitement |
|---|---|---|
| `spec.portalRef` | `spec.portalRef` | copie directe |
| `spec.sourceType` | `spec.sourceType` | copie directe |
| `spec.origin` | — | Ignoré (pas de champ en v1alpha1) |
| `spec.entries[]` | — | Perdus si `origin=manual` (pas de champ équivalent) |
| `status.endpoints` | `status.endpoints` | copie directe |

### 6bis.3 Implémentation

Commande Kubebuilder pour le webhook de conversion :

```bash
kubebuilder create webhook --group sreportal --version v1alpha1 --kind DNS --conversion
kubebuilder create webhook --group sreportal --version v1alpha1 --kind DNSRecord --conversion
```

Structure résultante :

```
api/
  v1alpha1/
    dns_types.go           -- ConvertTo / ConvertFrom implémentés ici
    dns_conversion.go      -- généré par kubebuilder (scaffold)
    dnsrecord_types.go
    dnsrecord_conversion.go
  v1alpha2/
    dns_types.go           -- Hub() + nouveau schéma
    dnsrecord_types.go     -- Hub() + nouveau schéma
internal/
  webhook/v1alpha1/
    dns_webhook.go         -- validating/defaulting inchangé (version agnostique)
    dnsrecord_webhook.go   -- nouveau (§6.3)
```

Le webhook de conversion est servi sur le même endpoint que les autres webhooks (`/convert`) via `ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.DNS{}).Complete()`.

### 6bis.4 Annotation de préservation des groupes

L'annotation `sreportal.io/v1alpha1-groups` est un détail d'implémentation de la conversion. Elle est :
- Ajoutée par `ConvertTo` uniquement si `spec.groups` non vide.
- Lue par `ConvertFrom` pour restaurer l'état v1alpha1 (rollback).
- Lue par l'outil de migration `hack/migrate-dns-v2` pour créer les DNSRecord manuels.
- Supprimée par l'outil de migration après création réussie des DNSRecord.

---

## 7. Plan d'implémentation (ordre suggéré)

1. **Package v1alpha2** : `kubebuilder create api --group sreportal --version v1alpha2 --kind DNS --resource --controller=false` + idem `DNSRecord`. Définir les nouveaux types (§2, §3) dans `api/v1alpha2/`. Ajouter `Hub()` sur les deux types.
2. **Conversion webhook scaffold** : `kubebuilder create webhook --group sreportal --version v1alpha1 --kind DNS --conversion` + idem `DNSRecord`. Implémenter `ConvertTo` / `ConvertFrom` (§6bis).
3. **Webhook validating DNSRecord** : `kubebuilder create webhook --group sreportal --version v1alpha2 --kind DNSRecord --programmatic-validation`. Implémenter §6.3.
4. **Webhook validating DNS** : étendre `internal/webhook/v1alpha2/dns_webhook.go` (rule name=portalRef, immutabilité).
5. **`make manifests generate`** : régénère CRDs multi-version + deepcopy pour v1alpha1 et v1alpha2.
6. **Chains DNSRecord** : ajouter `LoadDNSConfigHandler`, `MaterialiseManualEntriesHandler`, adapter `ProjectStoreHandler` pour `SourceManual`.
7. **Chains DNS** : remplacer la chaîne actuelle par `ListDNSRecordsHandler` → `AggregateStatusHandler` → `UpdateStatusHandler`.
8. **SourceReconciler** : refactor vers `configs map[string]*ResolvedDNSConfig` ; ajouter `DNSConfigReconciler` + channel de notification.
9. **cmd/main.go** : retirer lecture sections `sources`/`groupMapping`/`reconciliation` du ConfigMap ; importer v1alpha2 ; câbler `DNSConfigReconciler`.
10. **Tests** : Ginkgo envtest pour conversion webhook, dual-purpose DNSRecord, 1:1 strict, reconfig à chaud.
11. **Outil migration** : `hack/migrate-dns-v2/main.go` (lit annotation `sreportal.io/v1alpha1-groups`, crée DNSRecord manuels, patche `spec.sources` depuis ConfigMap).
12. **Sample CRs** : remplacer `config/samples/sreportal_v1alpha2_dns.yaml` + ajouter `sreportal_v1alpha2_dnsrecord_manual.yaml`.
13. **Helm/Docs** : `make helm` + `make doc`.

---

## 8. Exemple de manifests cibles

### 8.1 DNS CR (nouvelle forme)

```yaml
apiVersion: sreportal.io/v1alpha1
kind: DNS
metadata:
  name: main           # MUST equal spec.portalRef
  namespace: default
spec:
  portalRef: main
  sources:
    ingress:
      enabled: true
      namespace: ""
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
      devops: DevOps
  reconciliation:
    interval: 5m
    retryOnError: 30s
    disableDNSCheck: false
```

### 8.2 DNSRecord manuel (remplace l'ancien `spec.groups[]`)

```yaml
apiVersion: sreportal.io/v1alpha1
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

### 8.3 DNSRecord auto (généré par le controller — inchangé visuellement)

```yaml
apiVersion: sreportal.io/v1alpha1
kind: DNSRecord
metadata:
  name: main-ingress
  namespace: default
spec:
  origin: auto
  portalRef: main
  sourceType: ingress
```

---

## 9. Risques & points d'attention

1. **Cache controller-runtime sur DNS** : `r.List(ctx, &dnsList)` dans le ticker doit utiliser le cache (pas l'API server). S'assurer que `DNSReconciler` et `DNSConfigReconciler` sont enregistrés (sinon le cache n'est pas peuplé).
2. **Reconfiguration coûteuse** : `BuildTypedSources` instancie des informers external-dns. Une modification `spec.sources` fréquente peut être coûteuse. Garde-fou : debouncer 5s sur `DNSConfigReconciler.Notify`.
3. **Coexistence v1alpha1 ↔ v1alpha1 (breaking)** : pas de conversion webhook nécessaire (même API version, mais schéma changé). Migration impérative **avant** apply de la nouvelle CRD.
4. **Webhook `origin=auto` réservé au controller** : risque de blocage si le ServiceAccount opérateur change de nom. Externaliser le nom via env var `SREPORTAL_CONTROLLER_SA`.
5. **`status.activeSources`** : doit être calculé après `RebuildSources`, attention au race entre tick et patch de status.
6. **Memory pressure des informers** : N DNS CR × M source types = N×M informers external-dns. Pour limiter, considérer un cache d'informer mutualisé par GVR partagé entre les DNS qui ont la même config source (optimisation future, non bloquante).

---

## 10. Synthèse des changements fichier-par-fichier

| Fichier | Action |
|---|---|
| `api/v1alpha2/` (nouveau package) | `DNS` + `DNSRecord` v1alpha2 avec `Hub()`, `+kubebuilder:storageversion` |
| `api/v1alpha2/dns_types.go` (nouveau) | Nouveau schéma §2 — contient tous les sous-types inline (`SourcesSpec`, `*SourceSpec`, `GroupMappingSpec`, `ReconciliationSpec`) |
| `api/v1alpha2/dnsrecord_types.go` (nouveau) | Nouveau schéma §3 |
| `api/v1alpha1/dns_types.go` | Ajouter `ConvertTo` / `ConvertFrom` (§6bis) + annotation `sreportal.io/v1alpha1-groups` |
| `api/v1alpha1/dns_conversion.go` (généré) | Scaffold kubebuilder |
| `api/v1alpha1/dnsrecord_types.go` | Ajouter `ConvertTo` / `ConvertFrom` (§6bis) |
| `api/v1alpha1/dnsrecord_conversion.go` (généré) | Scaffold kubebuilder |
| `internal/webhook/v1alpha2/dns_webhook.go` (nouveau) | Validating : name=portalRef, immutabilité, Portal exists |
| `internal/webhook/v1alpha2/dnsrecord_webhook.go` (nouveau) | Validating : Portal exists, DNS exists, `origin=auto` réservé SA |
| `internal/controller/dns/dns_controller.go` | Importer v1alpha2 ; simplifier (§4.3) |
| `internal/controller/dns/chain/*.go` | Supprimer handlers manuels ; ajouter `ListDNSRecordsHandler`, `AggregateStatusHandler` |
| `internal/controller/dnsrecords/dnsrecord_controller.go` | Importer v1alpha2 ; ajouter `LoadDNSConfigHandler`, `MaterialiseManualEntriesHandler` |
| `internal/controller/dnsrecords/chain/load_dns_config.go` (nouveau) | Charger DNS CR v1alpha2 du portal |
| `internal/controller/dnsrecords/chain/materialise_manual.go` (nouveau) | `spec.entries[]` → `status.endpoints` |
| `internal/controller/dnsrecords/chain/project_store.go` | Lire groupMapping depuis ChainData ; `SourceManual` si `origin=manual` |
| `internal/controller/source/source_controller.go` | Importer v1alpha2 ; refactor map per-DNS (§4.1) |
| `internal/controller/source/dns_config_notifier.go` (nouveau) | Controller minimal de notification (§4.1) |
| `internal/controller/source/chain/build_portal_index.go` | Filtrer sur portal courant uniquement |
| `cmd/main.go` | Importer v1alpha2 ; retirer config sources/group/reconcile ; câbler `DNSConfigReconciler` + channel |
| `config/samples/sreportal_v1alpha2_dns.yaml` (nouveau) | §8.1 — remplace l'ancien sample v1alpha1 |
| `config/samples/sreportal_v1alpha2_dnsrecord_manual.yaml` (nouveau) | §8.2 |
| `hack/migrate-dns-v2/main.go` (nouveau) | Lit annotation `sreportal.io/v1alpha1-groups`, crée DNSRecord manuels, patche sources |
| `CLAUDE.md` | Mettre à jour : v1alpha2 pour DNS/DNSRecord, suppression `spec.groups`, suppression ConfigMap sources |

---

Fin du spec.
