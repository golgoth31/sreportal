# Plan : Feature Toggles par Portal

## Contexte

Aujourd'hui, la visibilité des pages (releases, alerts) est déterminée par des heuristiques côté frontend (portal.main + hasReleases, portalNamesWithAlerts). Les controllers tournent toujours, les endpoints gRPC/MCP servent toujours les données. On veut un contrôle explicite par portal via 4 booléens dans le CRD : `dns`, `releases`, `networkPolicy`, `alerts`.

## Approche

Ajouter un champ `features` sur `PortalSpec` avec 4 `*bool` (nil = true). Propager à travers : webhook defaulting → domain read model → proto → gRPC → web UI. Côté controllers, skip la réconciliation quand la feature est désactivée pour le portal référencé.

---

## Étape 1 : CRD — `api/v1alpha1/portal_types.go`

Ajouter `PortalFeatures` struct et champ sur `PortalSpec` :

```go
type PortalFeatures struct {
    // +optional
    // +kubebuilder:default=true
    DNS *bool `json:"dns,omitempty"`
    // +optional
    // +kubebuilder:default=true
    Releases *bool `json:"releases,omitempty"`
    // +optional
    // +kubebuilder:default=true
    NetworkPolicy *bool `json:"networkPolicy,omitempty"`
    // +optional
    // +kubebuilder:default=true
    Alerts *bool `json:"alerts,omitempty"`
}
```

Champ sur `PortalSpec` :
```go
// +optional
Features *PortalFeatures `json:"features,omitempty"`
```

Ajouter helper nil-safe sur `PortalFeatures` :
```go
func (f *PortalFeatures) IsDNSEnabled() bool           { return f == nil || f.DNS == nil || *f.DNS }
func (f *PortalFeatures) IsReleasesEnabled() bool      { return f == nil || f.Releases == nil || *f.Releases }
func (f *PortalFeatures) IsNetworkPolicyEnabled() bool  { return f == nil || f.NetworkPolicy == nil || *f.NetworkPolicy }
func (f *PortalFeatures) IsAlertsEnabled() bool         { return f == nil || f.Alerts == nil || *f.Alerts }
```

**Fichier** : `api/v1alpha1/portal_types.go`

## Étape 2 : Webhook defaulting — `internal/webhook/v1alpha1/portal_webhook.go`

Dans `PortalCustomDefaulter.Default`, après le subPath defaulting :

```go
if obj.Spec.Features == nil {
    obj.Spec.Features = &sreportalv1alpha1.PortalFeatures{}
}
t := true
if obj.Spec.Features.DNS == nil { obj.Spec.Features.DNS = &t }
if obj.Spec.Features.Releases == nil { obj.Spec.Features.Releases = &t }
if obj.Spec.Features.NetworkPolicy == nil { obj.Spec.Features.NetworkPolicy = &t }
if obj.Spec.Features.Alerts == nil { obj.Spec.Features.Alerts = &t }
```

**Fichier** : `internal/webhook/v1alpha1/portal_webhook.go`

## Étape 3 : Proto — `proto/sreportal/v1/portal.proto`

Ajouter message + champ :

```protobuf
message PortalFeatures {
  bool dns = 1;
  bool releases = 2;
  bool network_policy = 3;
  bool alerts = 4;
}

message Portal {
  // ... champs 1-9 existants ...
  PortalFeatures features = 10;
}
```

Puis `make proto` pour regénérer Go + TypeScript.

**Fichier** : `proto/sreportal/v1/portal.proto`

## Étape 4 : Domain read model — `internal/domain/portal/read_model.go`

Ajouter à `PortalView` :

```go
type PortalFeatures struct {
    DNS           bool
    Releases      bool
    NetworkPolicy bool
    Alerts        bool
}

// Dans PortalView :
Features PortalFeatures
```

**Fichier** : `internal/domain/portal/read_model.go`

## Étape 5 : Portal controller projection — `internal/controller/portal_controller.go`

### 5a. `portalToView` (ligne 389-415) — ajouter features :

```go
view.Features = domainportal.PortalFeatures{
    DNS:           p.Spec.Features.IsDNSEnabled(),
    Releases:      p.Spec.Features.IsReleasesEnabled(),
    NetworkPolicy: p.Spec.Features.IsNetworkPolicyEnabled(),
    Alerts:        p.Spec.Features.IsAlertsEnabled(),
}
```

### 5b. Remote CR creation — skip pour features désactivées

Dans la réconciliation des portals remote, conditionner la création de :
- Remote DNS CR → `portal.Spec.Features.IsDNSEnabled()`
- Remote Alertmanager CR → `portal.Spec.Features.IsAlertsEnabled()`
- Remote NetworkFlowDiscovery CR → `portal.Spec.Features.IsNetworkPolicyEnabled()`

Si feature désactivée et CR remote existant → le supprimer (cleanup).

**Fichier** : `internal/controller/portal_controller.go`

## Étape 6 : Controllers — skip réconciliation

### 6a. DNS controller (`internal/controller/dns_controller.go`)
Au début de `Reconcile`, après fetch du DNS resource : lookup du Portal via `resource.Spec.PortalRef`, vérifier `portal.Spec.Features.IsDNSEnabled()`. Si false → return early.

### 6b. DNSRecord controller (`internal/controller/dnsrecord_controller.go`)
Même pattern : lookup Portal via `record.Spec.PortalRef`, check `IsDNSEnabled()`.

### 6c. Source controller (`internal/controller/source/build_portal_index.go`)
Dans `BuildPortalIndexHandler.Handle`, filtrer les portals avec DNS désactivé hors de `idx.Local` (même pattern que le filtre Remote existant ligne 65-68).

### 6d. Alertmanager controller (`internal/controller/alertmanager_controller.go`)
Lookup Portal via `resource.Spec.PortalRef`, check `IsAlertsEnabled()`.

### 6e. NetworkFlowDiscovery controller (`internal/controller/networkflowdiscovery_controller.go`)
Lookup Portal via `resource.Spec.PortalRef`, check `IsNetworkPolicyEnabled()`.

### 6f. Release controller (`internal/controller/release_controller.go`)
Pas de changement — les releases sont globales (pas de portalRef). Le toggle contrôle uniquement la visibilité UI.

### 6g. EnsureNFDRunnable (`internal/controller/networkflowdiscovery/ensure_nfd.go`)
Skip la création du NFD si le main portal a networkPolicy désactivé.

## Étape 7 : gRPC portal service — `internal/grpc/portal_service.go`

Dans `portalViewToProto` (ligne 64-91), mapper features :

```go
portal.Features = &portalv1.PortalFeatures{
    Dns:           v.Features.DNS,
    Releases:      v.Features.Releases,
    NetworkPolicy: v.Features.NetworkPolicy,
    Alerts:        v.Features.Alerts,
}
```

**Note** : pas de changement nécessaire sur les autres services gRPC (DNS, Alertmanager, NetworkPolicy) ni MCP — les controllers ne pousseront pas de données dans le ReadStore pour les features désactivées, donc les réponses seront naturellement vides.

**Fichier** : `internal/grpc/portal_service.go`

## Étape 8 : Web UI

### 8a. Domain type — `web/src/features/portal/domain/portal.types.ts`

```typescript
export interface PortalFeatures {
  readonly dns: boolean;
  readonly releases: boolean;
  readonly networkPolicy: boolean;
  readonly alerts: boolean;
}

export interface Portal {
  // ... existant ...
  readonly features: PortalFeatures;
}
```

### 8b. API mapping — `web/src/features/portal/infrastructure/portalApi.ts`

Dans `toDomainPortal`, mapper `p.features` :
```typescript
features: {
  dns: p.features?.dns ?? true,
  releases: p.features?.releases ?? true,
  networkPolicy: p.features?.networkPolicy ?? true,
  alerts: p.features?.alerts ?? true,
},
```

### 8c. Sidebar — `web/src/components/PortalSidebar.tsx`

Remplacer les conditions actuelles :
- **DNS** (ligne 38-52, toujours affiché) → `currentPortal?.features.dns !== false`
- **Releases** (ligne 30, `currentPortal?.main && hasReleases`) → `currentPortal?.features.releases === true`
- **Network Policies** (ligne 69-82, toujours affiché) → `currentPortal?.features.networkPolicy !== false`
- **Alerts** (ligne 28-29, `portalNamesWithAlerts.has(name)`) → `currentPortal?.features.alerts === true`

Supprimer les props `portalNamesWithAlerts` et `hasReleases` du sidebar. Supprimer les hooks `useHasReleases` et `usePortalsWithAlerts` du `RootLayout.tsx` (plus nécessaires).

### 8d. Route guards

Ajouter un redirect vers `/links` si l'utilisateur navigue vers une page dont la feature est désactivée (dans le loader ou un layout guard).

## Étape 9 : Tests

| Scope | Fichier | Quoi tester |
|-------|---------|-------------|
| Unit | `api/v1alpha1/portal_types_test.go` | `IsXXXEnabled()` : nil features, nil field, true, false |
| Unit | `internal/webhook/v1alpha1/portal_webhook_test.go` | Defaulting peuple tous les flags à true |
| Unit | Controllers `_test.go` | Portal avec feature disabled → controller return early, pas de write au ReadStore |
| Unit | `internal/grpc/portal_service_test.go` | `portalViewToProto` mappe correctement features |
| Web | `web/src/components/PortalSidebar.test.tsx` | Nav items hidden quand feature disabled |

## Étape 10 : Make targets (dans l'ordre)

```bash
make helm       # Regénère CRDs, RBAC, Helm chart
make proto      # Regénère Go + TypeScript depuis proto mis à jour
make doc        # Met à jour la doc API
make test       # Tests unitaires
make lint       # golangci-lint
```

## Fichiers critiques à modifier

| Fichier | Changement |
|---------|-----------|
| `api/v1alpha1/portal_types.go` | PortalFeatures struct + helpers |
| `internal/webhook/v1alpha1/portal_webhook.go` | Defaulting features |
| `proto/sreportal/v1/portal.proto` | PortalFeatures message |
| `internal/domain/portal/read_model.go` | PortalFeatures dans PortalView |
| `internal/controller/portal_controller.go` | portalToView + skip remote CRs |
| `internal/controller/dns_controller.go` | Skip si DNS disabled |
| `internal/controller/dnsrecord_controller.go` | Skip si DNS disabled |
| `internal/controller/source/build_portal_index.go` | Filtrer portals DNS disabled |
| `internal/controller/alertmanager_controller.go` | Skip si alerts disabled |
| `internal/controller/networkflowdiscovery_controller.go` | Skip si networkPolicy disabled |
| `internal/controller/networkflowdiscovery/ensure_nfd.go` | Skip si networkPolicy disabled |
| `internal/grpc/portal_service.go` | Mapper features dans proto |
| `web/src/features/portal/domain/portal.types.ts` | PortalFeatures interface |
| `web/src/features/portal/infrastructure/portalApi.ts` | Mapper features |
| `web/src/components/PortalSidebar.tsx` | Conditions basées sur features |
| `web/src/components/RootLayout.tsx` | Supprimer hooks hasReleases/portalNamesWithAlerts |
