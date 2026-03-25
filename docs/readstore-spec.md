# ReadStore — Specification

## Objectif

Découpler les couches delivery (gRPC, MCP) de la réconciliation Kubernetes en introduisant un **ReadStore in-memory** alimenté par les controllers. Les services de lecture consomment des read models pré-agrégés, sans accès direct aux CRDs.

## Principes

- **CQRS léger** : les controllers écrivent des projections, les services gRPC/MCP lisent
- **Clean Architecture** : les interfaces Reader/Writer vivent dans `internal/domain`, l'implémentation in-memory dans `internal/readstore`
- **Transformation unique** : la conversion CRD → domain model se fait une seule fois au moment de la réconciliation
- **Notification event-driven** : les streams (ex: `StreamFQDNs`) utilisent un channel de notification au lieu de polling
- **Portabilité** : l'implémentation in-memory peut être remplacée par Redis/Memcached en changeant uniquement la couche infra

## Architecture

```
Controller (reconcile)
    │
    │ writer.Replace(key, readModels)
    ▼
┌──────────────────────────────────┐
│       ReadStore (infra)           │
│   map[string][]T + RWMutex       │
│   + broadcast channel (notify)    │
│                                   │
│   Implémente:                     │
│     domain.XxxReader              │
│     domain.XxxWriter              │
└──────────────────────────────────┘
    ▲
    │ reader.List(ctx, filters)
    │ reader.Subscribe()
    │
gRPC / MCP / Stream
```

## Bounded Contexts

### 1. DNS (FQDNs)

**Read model** — `internal/domain/dns/read_model.go`

```go
type FQDNView struct {
    Name         string
    Source       Source
    Groups       []string
    Description  string
    RecordType   string
    Targets      []string
    LastSeen     time.Time
    PortalName   string       // DNS CR name (== portal via portalRef)
    Namespace    string       // DNS CR namespace
    OriginRef    *ResourceRef
    SyncStatus   string
}

type FQDNFilters struct {
    Portal    string
    Namespace string
    Source    string
    Search    string // substring match on Name (case-insensitive)
}
```

**Interfaces** — `internal/domain/dns/reader.go` / `writer.go`

```go
type FQDNReader interface {
    List(ctx context.Context, filters FQDNFilters) ([]FQDNView, error)
    Get(ctx context.Context, name, recordType string) (FQDNView, error)
    Count(ctx context.Context, filters FQDNFilters) (int, error)
    Subscribe() <-chan struct{}
}

type FQDNWriter interface {
    Replace(ctx context.Context, resourceKey string, fqdns []FQDNView) error
    Delete(ctx context.Context, resourceKey string) error
}
```

**Consumers** : `grpc.DNSService` (ListFQDNs, StreamFQDNs), `mcp.DNSServer` (search_fqdns, get_fqdn_details)

**Producer** : `DNSReconciler` (après exécution de la chain, pousse `resource.Status.Groups` transformé)

### 2. Portal

**Read model** — `internal/domain/portal/read_model.go`

```go
type PortalView struct {
    Name       string
    Title      string
    Main       bool
    SubPath    string
    Namespace  string
    Ready      bool
    IsRemote   bool
    URL        string
    RemoteSync *RemoteSyncView
}

type RemoteSyncView struct {
    LastSyncTime  string
    LastSyncError string
    RemoteTitle   string
    FQDNCount     int
}

type PortalFilters struct {
    Namespace string
}
```

**Interfaces**

```go
type PortalReader interface {
    List(ctx context.Context, filters PortalFilters) ([]PortalView, error)
    Subscribe() <-chan struct{}
}

type PortalWriter interface {
    Replace(ctx context.Context, key string, portal PortalView) error
    Delete(ctx context.Context, key string) error
}
```

**Consumers** : `grpc.PortalService`, `mcp.DNSServer` (list_portals)

**Producer** : `PortalReconciler`

### 3. Alertmanager

**Read model** — `internal/domain/alertmanagerreadmodel/read_model.go`

```go
type AlertmanagerView struct {
    Name              string
    Namespace         string
    PortalRef         string
    LocalURL          string
    RemoteURL         string
    Ready             bool
    LastReconcileTime *time.Time
    Alerts            []AlertView
    Silences          []SilenceView
}

type AlertView struct {
    Fingerprint string
    Labels      map[string]string
    Annotations map[string]string
    State       string
    StartsAt    time.Time
    EndsAt      *time.Time
    UpdatedAt   time.Time
    Receivers   []string
    SilencedBy  []string
}

type SilenceView struct {
    ID        string
    Matchers  []MatcherView
    StartsAt  time.Time
    EndsAt    time.Time
    Status    string
    CreatedBy string
    Comment   string
    UpdatedAt time.Time
}

type MatcherView struct {
    Name    string
    Value   string
    IsRegex bool
}

type AlertmanagerFilters struct {
    Portal    string
    Namespace string
}
```

**Interfaces**

```go
type AlertmanagerReader interface {
    List(ctx context.Context, filters AlertmanagerFilters) ([]AlertmanagerView, error)
    Subscribe() <-chan struct{}
}

type AlertmanagerWriter interface {
    Replace(ctx context.Context, key string, view AlertmanagerView) error
    Delete(ctx context.Context, key string) error
}
```

**Consumers** : `grpc.AlertmanagerService`, `mcp.AlertsServer`

**Producer** : `AlertmanagerReconciler`

### 4. Network Policy (Flow Graph)

**Read model** — `internal/domain/netpol/read_model.go`

```go
type FlowNode struct {
    ID        string
    Label     string
    Namespace string
    NodeType  string
    Group     string
}

type FlowEdge struct {
    From     string
    To       string
    EdgeType string
}

type FlowGraphFilters struct {
    Portal    string
    Namespace string
    Search    string
}
```

**Interfaces**

```go
type FlowGraphReader interface {
    ListNodes(ctx context.Context, filters FlowGraphFilters) ([]FlowNode, error)
    ListEdges(ctx context.Context, filters FlowGraphFilters) ([]FlowEdge, error)
    Subscribe() <-chan struct{}
}

type FlowGraphWriter interface {
    ReplaceNodes(ctx context.Context, key string, nodes []FlowNode) error
    ReplaceEdges(ctx context.Context, key string, edges []FlowEdge) error
    Delete(ctx context.Context, key string) error
}
```

**Consumers** : `grpc.NetworkPolicyService`, `mcp.NetpolServer`

**Producer** : `NetworkFlowDiscoveryReconciler`

### 5. Release

**Particularité** : seul bounded context avec un write path gRPC direct (AddRelease → K8s). Le ReadStore ne couvre que la lecture.

**Read model** — `internal/domain/release/read_model.go`

```go
type EntryView struct {
    Type    string
    Version string
    Origin  string
    Date    time.Time
    Author  string
    Message string
    Link    string
}

type DayView struct {
    Day     string       // "2026-03-25"
    Entries []EntryView
}
```

**Interfaces**

```go
type ReleaseReader interface {
    ListEntries(ctx context.Context, day string) ([]EntryView, error)
    ListDays(ctx context.Context) ([]string, error)
    Subscribe() <-chan struct{}
}

type ReleaseWriter interface {
    Replace(ctx context.Context, day string, entries []EntryView) error
    Delete(ctx context.Context, day string) error
}
```

**Consumers** :
- `grpc.ReleaseService.ListReleases` / `ListReleaseDays` → `ReleaseReader`
- `grpc.ReleaseService.AddRelease` → `release.Service` (K8s direct, inchangé)

**Producer** : `ReleaseReconciler`

**Impact** : `release.Service` perd ses responsabilités cache (plus de `InvalidateDay`, `InvalidateDays`, plus de `map[string]*CachedDay`). Il ne garde que `AddEntry`.

## Store générique

`internal/readstore/store.go` — implémentation in-memory réutilisée par tous les bounded contexts.

```go
type Store[T any] struct {
    mu        sync.RWMutex
    data      map[string][]T  // resourceKey → read models

    notifyMu  sync.Mutex
    notifyCh  chan struct{}
}

func New[T any]() *Store[T]

// Replace atomically swaps all entries for a key and broadcasts.
func (s *Store[T]) Replace(key string, items []T)

// Delete removes a key and broadcasts.
func (s *Store[T]) Delete(key string)

// All returns a flat snapshot of all values across all keys.
func (s *Store[T]) All() []T

// Subscribe returns a channel closed on next mutation.
// Caller must call Subscribe() again after each notification.
func (s *Store[T]) Subscribe() <-chan struct{}
```

Chaque bounded context wraps ce store avec sa logique de filtrage/tri dans `internal/readstore/<context>/`.

## Notification (streams)

Le pattern broadcast channel remplace le polling :

1. Le store expose `Subscribe() <-chan struct{}`
2. À chaque `Replace` ou `Delete`, le channel courant est fermé (broadcast) et remplacé par un nouveau
3. `StreamFQDNs` fait un `select` sur le channel au lieu de poller toutes les 5s
4. Ce mécanisme est identique à celui déjà implémenté dans `DNSService.cacheUpdate`, mais mutualisé

## Wiring (cmd/main.go)

**État actuel** (DNS, Portal, Alertmanager, Release migrés) :

```go
// ✅ ReadStores — créés et câblés
fqdnStore := dnsreadstore.NewFQDNStore()
portalStore := portalreadstore.NewPortalStore()
alertmanagerStore := alertmanagerreadstore.NewAlertmanagerStore()
releaseStore := releasereadstore.NewReleaseStore()

// Controllers reçoivent les writers
dnsReconciler.SetFQDNWriter(fqdnStore)
portalReconciler.SetPortalWriter(portalStore)
amReconciler.SetAlertmanagerWriter(alertmanagerStore)
releaseReconciler.SetReleaseWriter(releaseStore)

// gRPC/MCP reçoivent les readers
webCfg.FQDNReader = fqdnStore
webCfg.PortalReader = portalStore
webCfg.AlertmanagerReader = alertmanagerStore
webCfg.ReleaseReader = releaseStore
mcp.NewDNSServer(fqdnStore, portalStore)
mcp.NewAlertsServer(alertmanagerStore)
mcp.NewReleasesServer(releaseStore)
```

**État cible** (tous les contextes migrés) :

```go
fqdnStore := dnsreadstore.NewFQDNStore()
portalStore := portalreadstore.NewPortalStore()
alertmanagerStore := alertmanagerreadstore.NewAlertmanagerStore()
flowStore := netpolreadstore.NewFlowGraphStore()
releaseStore := releasereadstore.NewReleaseStore()

// Controllers reçoivent les writers
dnsReconciler.SetFQDNWriter(fqdnStore)
portalReconciler.SetPortalWriter(portalStore)
amReconciler.SetAlertmanagerWriter(alertmanagerStore)
nfdReconciler.SetFlowGraphWriter(flowStore)
releaseReconciler.SetReleaseWriter(releaseStore)

// gRPC/MCP reçoivent les readers
webCfg.FQDNReader = fqdnStore
webCfg.PortalReader = portalStore
webCfg.AlertmanagerReader = alertmanagerStore
// ...
mcp.NewDNSServer(fqdnStore, portalStore)
mcp.NewAlertsServer(alertmanagerStore)
```

## Migration progressive

L'ordre recommandé est :

1. ✅ **Store générique** (`internal/readstore/store.go`) — fondation
2. ✅ **DNS** — le plus complexe (Stream + cache existant à supprimer), valide le pattern
3. ✅ **Portal** — simple, peu de logique
4. ✅ **Alertmanager** — filtrage modéré
5. ✅ **Release** — cas spécial write direct, suppression du cache artisanal
6. ⬜ **Network Policy** — merge multi-CRD (FlowNodeSet + FlowEdgeSet)

Chaque migration est indépendante et peut être livrée en PR séparée.

### Détail de la migration DNS (✅ terminée)

**Fichiers créés :**
- `internal/readstore/store.go` + `store_test.go` — store générique `Store[T]` (11 tests)
- `internal/domain/dns/read_model.go` — `FQDNView`, `FQDNFilters`
- `internal/domain/dns/reader.go` — interface `FQDNReader`
- `internal/domain/dns/writer.go` — interface `FQDNWriter`
- `internal/domain/dns/errors.go` — `ErrFQDNNotFound`
- `internal/readstore/dns/fqdn_store.go` + `fqdn_store_test.go` — implémentation `FQDNStore` (13 tests)
- `internal/grpc/helpers_test.go` — helper test partagé `newScheme`

**Fichiers modifiés :**
- `internal/controller/dns_controller.go` — ajout `fqdnWriter`, `SetFQDNWriter()`, `groupsToFQDNViews()` ; pousse dans le store après réconciliation
- `internal/grpc/dns_service.go` — **réécrit** : `FQDNReader` remplace `client.Client`, suppression du cache custom (`Start`, `refreshCache`, `fetchAllFQDNs`, `snapshotCache`, `cacheUpdate`, polling), `StreamFQDNs` utilise `Subscribe()`
- `internal/grpc/dns_service_test.go` — **réécrit** : utilise `FQDNStore` au lieu du fake K8s client
- `internal/mcp/server.go` — `DNSServer` reçoit `FQDNReader` + `PortalReader` (plus de `client.Client`)
- `internal/mcp/search_fqdns.go` — **réécrit** : utilise `FQDNReader.List()`
- `internal/mcp/get_fqdn_details.go` — **réécrit** : utilise `FQDNReader.Get()`, fallback DNSRecord supprimé
- `internal/mcp/mcp_test.go` — **réécrit** : tests DNS utilisent `FQDNStore`, tests Portal utilisent `PortalStore`
- `internal/webserver/server.go` — `Config.FQDNReader` ajouté, `dnsService` field et `DNSService()` supprimés
- `cmd/main.go` — création `fqdnStore`, wiring writer→controller, reader→webserver/MCP, suppression `mgr.Add(webServer.DNSService())`

**Supprimé :**
- `DNSService.Start()`, `refreshCache()`, `fetchAllFQDNs()`, `snapshotCache()`, `currentUpdate()`, `applyStreamFilters()`
- `cacheMu`, `cacheAll`, `cacheReady`, `cacheUpdate`, `cacheUpdateMu`, `streamPollInterval`
- `Server.dnsService` field et `Server.DNSService()` method dans webserver
- `DNSServer.groupMapping` field dans MCP
- Fallback DNSRecord dans `get_fqdn_details.go`
- `mgr.Add(webServer.DNSService())` dans `cmd/main.go`

### Détail de la migration Portal (✅ terminée)

**Fichiers créés :**
- `internal/domain/portal/read_model.go` — `PortalView`, `RemoteSyncView`, `PortalFilters`
- `internal/domain/portal/reader.go` — interface `PortalReader`
- `internal/domain/portal/writer.go` — interface `PortalWriter`
- `internal/readstore/portal/portal_store.go` + `portal_store_test.go` — implémentation `PortalStore` (5 tests)

**Fichiers modifiés :**
- `internal/controller/portal_controller.go` — ajout `portalWriter`, `SetPortalWriter()`, `portalToView()` ; pousse dans le store après réconciliation, supprime du store en cas de NotFound
- `internal/grpc/portal_service.go` — **réécrit** : `PortalReader` remplace `client.Client`, `portalViewToProto()` helper
- `internal/mcp/server.go` — `DNSServer` reçoit `PortalReader` au lieu de `client.Client`
- `internal/mcp/list_portals.go` — **réécrit** : utilise `portalReader.List()`
- `internal/mcp/mcp_test.go` — tests Portal récrits avec `PortalStore`, suppression du test "client errors" (store ne retourne jamais d'erreur), `NewDNSServer` prend `portalStore` au lieu de `k8sClient`
- `internal/webserver/server.go` — ajout `PortalReader` dans `Config`, utilisé pour `grpc.NewPortalService()`
- `cmd/main.go` — création `portalStore`, wiring writer→controller, reader→webserver/MCP

**Supprimé :**
- `client.Client` dans `DNSServer` (MCP) — remplacé par `PortalReader`
- Imports `client`, `interceptor`, `errors` dans `mcp_test.go` (devenus inutiles pour les tests portail)

### Détail de la migration Alertmanager (✅ terminée)

**Fichiers créés :**
- `internal/domain/alertmanagerreadmodel/read_model.go` — `AlertmanagerView`, `AlertView`, `SilenceView`, `MatcherView`, `AlertmanagerFilters`
- `internal/domain/alertmanagerreadmodel/reader.go` — interface `AlertmanagerReader`
- `internal/domain/alertmanagerreadmodel/writer.go` — interface `AlertmanagerWriter`
- `internal/readstore/alertmanager/alertmanager_store.go` + `alertmanager_store_test.go` — implémentation `AlertmanagerStore` (7 tests)

**Fichiers modifiés :**
- `internal/controller/alertmanager_controller.go` — ajout `alertmanagerWriter`, `SetAlertmanagerWriter()`, `alertmanagerToView()` ; pousse dans le store après réconciliation, supprime du store en cas de NotFound
- `internal/grpc/alertmanager_service.go` — **réécrit** : `AlertmanagerReader` remplace `client.Client`, fonctions renommées (`alertViewsToProto`, `silenceViewsToProto`, `matchesAlertViewSearch`)
- `internal/grpc/alertmanager_service_test.go` — **réécrit** : utilise `AlertmanagerStore` au lieu du fake K8s client
- `internal/mcp/alerts_server.go` — **réécrit** : `AlertmanagerReader` remplace `client.Client`, suppression imports CRD/K8s
- `internal/webserver/server.go` — ajout `AlertmanagerReader` dans `Config`, utilisé pour `grpc.NewAlertmanagerService()`
- `cmd/main.go` — création `alertmanagerStore`, wiring writer→controller, reader→webserver/MCP

**Supprimé :**
- `client.Client` dans `AlertsServer` (MCP) et `AlertmanagerService` (gRPC)
- Imports `sreportalv1alpha1`, `sigs.k8s.io/controller-runtime/pkg/client` dans les services alertmanager

### Détail de la migration Release (✅ terminée)

**Fichiers créés :**
- `internal/domain/release/read_model.go` — `EntryView`, `DayView`
- `internal/domain/release/reader.go` — interface `ReleaseReader`
- `internal/domain/release/writer.go` — interface `ReleaseWriter`
- `internal/readstore/release/release_store.go` + `release_store_test.go` — implémentation `ReleaseStore` (5 tests)

**Store générique étendu :**
- `internal/readstore/store.go` — ajout `Get(key)` et `Keys()` (5 tests ajoutés)

**Fichiers modifiés :**
- `internal/controller/release_controller.go` — suppression `CacheInvalidator`, ajout `releaseWriter`, `SetReleaseWriter()`, `releaseEntriesToViews()` ; pousse dans le store après réconciliation, supprime du store en cas de NotFound
- `internal/controller/release_controller_test.go` — réécrit avec `ReleaseStore` au lieu de `fakeCacheInvalidator`, vérifie le contenu du store
- `internal/grpc/release_service.go` — `ReleaseReader` remplace `release.Service` pour les lectures (`ListReleases`, `ListReleaseDays`), `release.Service` conservé pour `AddRelease` (write path K8s)
- `internal/grpc/release_service_test.go` — tests lecture utilisent `ReleaseStore`, tests écriture conservent fake K8s client
- `internal/mcp/releases_server.go` — `ReleaseReader` remplace `release.Service`
- `internal/mcp/mcp_test.go` — tests Release récrits avec `ReleaseStore`
- `internal/release/service.go` — suppression `ListEntries`, `ListDays`, `InvalidateDay`, `InvalidateDays`, et tout le cache (`CachedDay`, `mu`, `cache`, `daysMu`, `daysCache`, `daysValid`)
- `internal/release/service_test.go` — conserve uniquement les tests `AddEntry`
- `internal/webserver/server.go` — ajout `ReleaseReader` dans `Config`
- `cmd/main.go` — création `releaseStore`, wiring writer→controller, reader→webserver/MCP

**Supprimé :**
- `CacheInvalidator` interface dans controller
- `release.Service.ListEntries()`, `ListDays()`, `InvalidateDay()`, `InvalidateDays()`
- `CachedDay` type, `mu`, `cache`, `daysMu`, `daysCache`, `daysValid` dans `release.Service`
- Imports `metav1`, `fake` dans `mcp_test.go` (devenus inutiles pour les tests release)

## Tableau récapitulatif

| Bounded Context | Write path | Read path | Controller → Store | Statut |
|---|---|---|---|---|
| DNS | Reconciliation CRD | `FQDNReader` | `Replace(resourceKey, []FQDNView)` | ✅ Done |
| Portal | Reconciliation CRD | `PortalReader` | `Replace(key, PortalView)` | ✅ Done |
| Alertmanager | Reconciliation CRD | `AlertmanagerReader` | `Replace(key, AlertmanagerView)` | ✅ Done |
| Netpol | Reconciliation CRD | `FlowGraphReader` | `ReplaceNodes/ReplaceEdges(key, ...)` | ⬜ TODO |
| Release | **AddEntry → K8s direct** | `ReleaseReader` | `Replace(day, []EntryView)` | ✅ Done |
