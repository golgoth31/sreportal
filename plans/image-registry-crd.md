# Plan — ImageRegistry CRD + registry version lookup

Statut : design validé, à implémenter.

Ce plan couvre deux fonctions liées :

1. **F1** — Pour chaque image observée dans le cluster, interroger la registry pour récupérer la dernière version semver disponible (hors `latest`).
2. **F2** — CRD `ImageRegistry` qui agrège, par `(portal, host, namespace)`, les images détectées par `ImageInventory`, avec pour chaque entrée : image d'origine (template PodSpec), image mutée (Pod observé), type de changement, et dernière version disponible sur la registry d'origine.

---

## 0. Architecture (modèle inversé)

```
┌─────────────────────────────────────────────────────────────────┐
│ ImageInventory controller                                       │
│   • watch cluster (Pods + workload templates)                   │
│   • compute change_type per (portal, host, namespace) entry     │
│   • create/update ImageRegistry CRs (ownerRef = ImageInventory) │
│   • Status.Registries[] : lookup table {hash, portal, host, ns} │
│   • purge ImageRegistry CRs whose host disappeared              │
└─────────────────────────────┬───────────────────────────────────┘
                              │ writes Spec
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ ImageRegistry CR  (one per (portal, host, namespace))           │
│   Spec  : Host, PortalRef, Namespace, Images[]                  │
│           (managed by ImageInventory, not user-edited in v1)    │
│   Status: per-image LastCheckedAt, LatestVersion, LatestError   │
└─────────────────────────────┬───────────────────────────────────┘
                              │ read by
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ ImageRegistry controller                                        │
│   • read own Spec.Images                                        │
│   • for each image due (LastCheckedAt+Interval < now):          │
│       lookup latest tag via registry origin (or injected)       │
│   • token bucket per host (hardcoded limits)                    │
│   • startup catch-up jitter (β): if > N% due, spread over       │
│     [0, interval] instead of immediate enqueue                  │
│   • write readstore (single writer)                             │
│   • patch Status with per-image LastCheckedAt + LatestVersion   │
└─────────────────────────────┬───────────────────────────────────┘
                              │ writes
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Readstore (in-memory)                                           │
│   • dedup global by (portal, registry, repo, tag)               │
│   • namespace = info field on workloads, not key                │
│   • consumed by gRPC API (Web UI, MCP)                          │
└─────────────────────────────────────────────────────────────────┘
```

**Différence clé vs intuition initiale** : le readstore n'est pas l'état pivot. C'est la `Spec` des CR `ImageRegistry` qui pivote `ImageInventory` (producteur de spec) et le controller `ImageRegistry` (résolveur registry, écrivain readstore).

---

## 1. Exploration de l'existant

### 1.1 Controller `ImageInventory`

`internal/controller/imageinventory/`

Chain actuelle :

```
ValidateSpec → ValidatePortalRef → FetchRemoteImages (remote only) →
ScanWorkloads (local only) → ProjectImages → UpdateStatus
```

- `ChainData.ByWorkload` : `map[WorkloadKey][]ImageView`, alimenté par scan local ou fetch remote, écrit dans le readstore via `ProjectImagesHandler`.
- `ImageView` (`internal/domain/image/read_model.go`) :
  ```go
  type ImageView struct {
      PortalRef  string
      Registry   string
      Repository string
      Tag        string
      TagType    TagType
      Workloads  []WorkloadRef
  }
  ```
- `TagType` ∈ `{semver, commit, digest, latest, other}` (`internal/domain/image/parser.go`).
- `WorkloadRef.Source` ∈ `{spec, pod}` distingue l'image template de celle observée.

### 1.2 Pattern de référence — Alertmanager

`internal/controller/alertmanager/` : chain `validate → fetch → update_status`, indexer field `spec.portalRef`, finalizer pour cleanup métriques, RemoteTLSConfig partagée.

### 1.3 Dépendances

- `github.com/google/go-containerregistry v0.21.5` — `pkg/v1/remote.List` pour `ListTags`, `authn.DefaultKeychain` pour l'auth (lit `~/.docker/config.json`, IRSA, GCR creds).
- `golang.org/x/mod/semver` — pour comparaison/sélection (à ajouter si pas déjà présent).
- `golang.org/x/time/rate` — token bucket par host (à ajouter).

### 1.4 API & UI

- Connect API : `proto/sreportal/v1/image.proto` expose `Image`. `RemoteImage` mirror dans `internal/remoteclient/`.
- Web UI : page Images existante dans `web/src/features/image/`.
- MCP : `internal/mcp/image_server.go` monté à `/mcp/image`.

---

## 2. Modifications du controller `ImageInventory`

`ImageInventory` gagne deux responsabilités nouvelles :

1. Calculer le `ChangeType` par container (cf. §2.1).
2. Créer / mettre à jour / supprimer les CR `ImageRegistry` enfants (cf. §2.2).

Le projection vers le readstore (`ProjectImagesHandler`) **disparaît** : c'est désormais le controller `ImageRegistry` qui écrit le readstore.

### 2.1 Calcul de `ChangeType`

Pour chaque container observé dans un workload :

| Cas | `originalImage` | `mutatedImage` | `ChangeType` |
|---|---|---|---|
| Container présent dans template, image inchangée pod | `nginx:1.25` | `nginx:1.25` | `none` |
| Container présent dans template, image réécrite par webhook | `nginx:1.25` | `mirror.io/nginx:1.25` | `mutated` |
| Container absent du template (sidecar injecté) | `""` | `istio/proxy:1.20` | `injected` |

L'appariement spec ↔ pod se fait **par nom de container** dans le même Pod. Init containers traités identiquement.

### 2.2 Nouveau handler `SyncRegistryCRs`

Inséré dans la chain `ImageInventory` après `ScanWorkloads` (à la place de `ProjectImages`) :

```
ValidateSpec → ValidatePortalRef → FetchRemoteImages (remote) →
ScanWorkloads (local) → SyncRegistryCRs → UpdateStatus
```

- **Skip si `IsRemote=true`** : un shadow ne crée pas de CR `ImageRegistry`. Les images remote consomment `latestVersion` directement depuis la projection remote (cf. §10).
- Agrège les containers par `(portal, host, namespace)`.
- Calcule le hash de nom : `sha256("{portal}|{host}|{namespace}")[:12]` en hex.
- Pour chaque groupe :
  - Si CR n'existe pas → `Create` (avec ownerRef vers `ImageInventory`).
  - Si CR existe → `Patch` de `Spec.Images` (préserve `Status`).
- Pour chaque CR existant qui n'a plus de groupe correspondant → `Delete`.
- Met à jour `ImageInventory.Status.Registries[]` (table de correspondance hash ↔ contexte).

### 2.3 Status augmenté de `ImageInventory`

```go
type ImageInventoryStatus struct {
    // ... existant ...
    Registries []ImageRegistryRef `json:"registries,omitempty"`
}

type ImageRegistryRef struct {
    Hash      string `json:"hash"`      // 12-char sha256 hex (= nom du CR)
    Host      string `json:"host"`
    Namespace string `json:"namespace"`
}
```

`PortalRef` est implicite (= `ImageInventory.Spec.PortalRef`).

---

## 3. CRD `ImageRegistry`

### 3.1 Création

```bash
kubebuilder create api    --group sreportal --version v1alpha1 --kind ImageRegistry
kubebuilder create webhook --group sreportal --version v1alpha1 --kind ImageRegistry \
  --defaulting --programmatic-validation
```

Namespaced.

### 3.2 Spec — `api/v1alpha1/imageregistry_types.go`

Toute la spec est **controller-managed** (par `ImageInventory`). Pas de champ user en v1 (auth, allowlist, semverConstraint, interval).

```go
type ImageRegistrySpec struct {
    // host : registry origine (ex. "docker.io", "ghcr.io").
    // +kubebuilder:validation:Required
    Host string `json:"host"`

    // portalRef : Portal dont cet inventaire est dérivé.
    // +kubebuilder:validation:Required
    PortalRef string `json:"portalRef"`

    // namespace : k8s namespace cible (NOT le namespace du CR lui-même).
    // +kubebuilder:validation:Required
    Namespace string `json:"namespace"`

    // images : liste des images détectées dans (portalRef, host, namespace).
    // +listType=map
    // +listMapKey=key
    Images []ImageRegistrySpecEntry `json:"images,omitempty"`
}

type ImageRegistrySpecEntry struct {
    // key : sha256(originalImage|mutatedImage|container)[:16] pour stabilité de listMapKey.
    // +kubebuilder:validation:Required
    Key string `json:"key"`

    // originalImage : ref dans PodSpec template. Vide si changeType=injected.
    OriginalImage string `json:"originalImage,omitempty"`

    // mutatedImage : ref observée dans le Pod runtime.
    // +kubebuilder:validation:Required
    MutatedImage string `json:"mutatedImage"`

    // changeType : none|mutated|injected.
    // +kubebuilder:validation:Enum=none;mutated;injected
    ChangeType string `json:"changeType"`

    // repository : ex. "library/nginx" (parsé depuis l'image cible du lookup).
    Repository string `json:"repository"`

    // originalTag : tag de l'image cible du lookup (originalImage si présent, sinon mutatedImage).
    OriginalTag string `json:"originalTag"`

    // tagType : semver|commit|digest|latest|other.
    TagType string `json:"tagType"`

    // workloads : workloads référençant cette entrée. Pas de cap en v1.
    Workloads []WorkloadRef `json:"workloads,omitempty"`
}

type WorkloadRef struct {
    Kind      string `json:"kind"`
    Namespace string `json:"namespace"` // info, peut différer si webhook cross-ns (rare)
    Name      string `json:"name"`
    Container string `json:"container"`
}
```

### 3.3 Status — `imageregistry_types.go` (suite)

```go
type ImageRegistryStatus struct {
    ObservedGeneration    int64        `json:"observedGeneration,omitempty"`
    LastError             string       `json:"lastError,omitempty"`

    // Compteurs résumés.
    ImageCount            int32 `json:"imageCount,omitempty"`
    UpgradeAvailableCount int32 `json:"upgradeAvailableCount,omitempty"`
    MutatedCount          int32 `json:"mutatedCount,omitempty"`
    InjectedCount         int32 `json:"injectedCount,omitempty"`

    // Détails par image (état authoritative pour reprise au démarrage).
    // +listType=map
    // +listMapKey=key
    Images []ImageRegistryStatusEntry `json:"images,omitempty"`

    // Conditions minimales : Ready (true/false). LastError porte le détail.
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ImageRegistryStatusEntry struct {
    // key : identique à Spec.Images[].Key (jointure).
    Key string `json:"key"`

    // latestVersion : plus haut tag semver disponible (vide si non applicable / erreur).
    LatestVersion string `json:"latestVersion,omitempty"`

    // upgradeAvailable : latestVersion existe et > originalTag.
    UpgradeAvailable bool `json:"upgradeAvailable,omitempty"`

    // lastCheckedAt : date du dernier appel registry réussi pour cette image.
    LastCheckedAt *metav1.Time `json:"lastCheckedAt,omitempty"`

    // lastError : dernière erreur de résolution (vide si succès).
    LastError string `json:"lastError,omitempty"`
}
```

`printcolumns` : `Host`, `Portal`, `Namespace`, `Images`, `Upgrades`, `Mutated`, `Injected`, `Age`.

### 3.4 Webhook validation — `internal/webhook/v1alpha1/imageregistry_webhook.go`

- Validation : `Host` parsable via `name.NewRegistry(host)`, `PortalRef` non vide, `Namespace` non vide, `Spec.Images[].MutatedImage` non vide.
- Defaulting : aucun (pas de champ optionnel user en v1).
- `ChangeType=injected` ⇒ `OriginalImage` doit être vide. `ChangeType ∈ {none, mutated}` ⇒ `OriginalImage` non vide.
- Validation `OriginalTag` cohérent avec l'image cible (regex tag valide).

---

## 4. Domaine — `internal/domain/imageregistry/`

### 4.1 Types

`types.go` : `Entry`, `WorkloadRef`, `ChangeType` (constantes `none|mutated|injected`).

### 4.2 Aggregator (utilisé par `ImageInventory.SyncRegistryCRs`)

`aggregator.go` :

```go
// AggregateForCRs takes raw scan results (containers from Pods + templates)
// and returns groups suitable for ImageRegistry CR Spec generation.
func AggregateForCRs(
    portalRef string,
    scan []ContainerObservation,
) map[Group][]Entry

type Group struct {
    Host      string
    Namespace string
}

type ContainerObservation struct {
    WorkloadKind      string
    WorkloadName      string
    WorkloadNamespace string
    ContainerName     string
    TemplateImage     string  // empty if injected
    PodImage          string
}
```

- Pour chaque observation : compute `changeType` → image cible du lookup (originalImage si non vide, sinon mutatedImage) → parse `(host, repo, tag)`.
- Group par `(host, namespace)`.
- Dedup par `(originalImage, mutatedImage, container)`, agrège les workloads.

### 4.3 Version

`version.go` (utilise `golang.org/x/mod/semver`) :

- `IsUpgradable(tt TagType) bool` → `true` uniquement pour `TagTypeSemver`.
- `PickLatestSemver(tags []string) (string, error)` :
  - Filtre les tags semver.
  - Ignore `"latest"`.
  - Retourne le plus haut. Préserve le préfixe `v` si l'input l'avait.

Pas de `SemverConstraint` en v1 (cf. décisions §17).

### 4.4 Hash naming

`naming.go` :

```go
// CRName returns sha256(portal|host|namespace)[:12] in hex (lowercase).
// Always 12 chars, conforme RFC 1123 (a-z, 0-9).
func CRName(portal, host, namespace string) string
```

### 4.5 Port registry

`registry_port.go` :

```go
type Client interface {
    ListTags(ctx context.Context, host, repository string) ([]string, error)
}
```

Pas d'options client en v1 (auth via `authn.DefaultKeychain` côté adapter).

---

## 5. Adapter registry — `internal/registry/`

- `crane_client.go` : implémente `domainimageregistry.Client` via `pkg/v1/remote.List`. Gère `404` (repo inexistant → tags vides + log debug). Gère `429` → erreur taggée `ErrRateLimited`.
- Auth : `authn.DefaultKeychain` (lit `~/.docker/config.json`, IRSA, GCR/ECR/ACR helpers). Pas de pull-secret K8s en v1.
- TLS : par défaut (pas de `InsecureSkipVerify` ni CA custom en v1).

### 5.1 Token bucket par host

`rate_limiter.go` :

```go
type HostLimiter struct {
    limiters map[string]*rate.Limiter // golang.org/x/time/rate
    defaultLim *rate.Limiter
    mu sync.RWMutex
}

func (l *HostLimiter) Wait(ctx context.Context, host string) error
```

Limites hardcodées (à ajuster selon retours prod) :

```go
var hostLimits = map[string]rate.Limit{
    "docker.io":              rate.Every(12 * time.Second), // 5/min
    "registry-1.docker.io":   rate.Every(12 * time.Second),
    "ghcr.io":                rate.Every(2 * time.Second),  // 30/min
    "registry.k8s.io":        rate.Every(2 * time.Second),
    "quay.io":                rate.Every(2 * time.Second),
    "gcr.io":                 rate.Every(2 * time.Second),
}
const defaultRate = rate.Every(3 * time.Second) // 20/min unknown hosts
```

Pas de cache de tags en v1 : le rate limiter + reconcile 1×/jour suffit. Si plusieurs CR du même host se font reconciler dans la même minute, le bucket les sérialise.

---

## 6. Controller — `internal/controller/imageregistry/`

Chain :

```
ValidateSpec → SelectDueImages → ResolveLatestVersions → UpdateReadstore → UpdateStatus
```

### 6.1 Reconciler

`imageregistry_controller.go` :

- Watche `ImageRegistry` (no watch sur readstore — ce controller en est l'écrivain unique).
- Field indexer `spec.portalRef`.
- Finalizer : nettoie compteurs métriques + retire les contributions du readstore.
- Interval reconcile = 24h (hardcoded) + jitter `±1h`.

### 6.2 Handlers

**`chain/validate_spec.go`** : `Host`, `PortalRef`, `Namespace` non vides. Sinon `ErrInvalidSpec`.

**`chain/select_due_images.go`** : pour chaque `Spec.Images[i]` :
- Récupère `Status.Images[i]` via key.
- Si `LastCheckedAt+24h < now` (ou jamais checked) → ajouter à `ChainData.DueImages`.
- Sinon skip.
- **Catch-up jitter** : si > 50% des images sont due **et** `Status` est non vide (= post-restart, pas un nouveau CR), répartir aléatoirement sur la fenêtre `[0, 24h]` au lieu de tout traiter immédiatement. Implémentation : pour chaque image due, tirer `delay = rand(0, 24h)` ; si `delay > 0`, on **skip ce cycle** (la prochaine reconcile dans `RequeueAfter` la retraitera). Le RequeueAfter du CR est réduit à `min(remainingDelays)` pour éviter de dormir 24h.

**`chain/resolve_latest_versions.go`** :
- Pour chaque `DueImage` :
  - Cible du lookup : `OriginalImage` si non vide, sinon `MutatedImage`.
  - Skip si `TagType != semver` → `LastCheckedAt = now`, pas de `LatestVersion`.
  - Sinon : `HostLimiter.Wait(host)` puis `Client.ListTags(host, repo)`.
  - `PickLatestSemver(tags)` → `LatestVersion`.
  - `UpgradeAvailable = semver.Compare(latest, originalTag) > 0`.
- Pas de fallback runtime sur erreur : si lookup origin échoue (mutated|none) → on remplit `LastError`, pas de retry sur registry mutée.
- Concurrence bornée intra-CR : `errgroup.SetLimit(4)`. Le rate limiter sérialise par host.

**`chain/update_readstore.go`** :
- Construit la liste d'`ImageView` (avec `LatestVersion`, `LatestCheckedAt`, etc.) à partir de `Spec.Images` + résultats résolus + état précédent (Status pour les non-due).
- Appelle `imageStore.ReplaceForNamespace(portalRef, host, namespace, views)`.

**`chain/update_status.go`** :
- Met à jour `Status.Images[]` (merge des résolutions du cycle avec celles non touchées).
- Recompute `ImageCount`, `UpgradeAvailableCount`, `MutatedCount`, `InjectedCount`.
- Patch `Conditions[Ready] = (LastError == "")`.
- `RequeueAfter` calculé selon les `LastCheckedAt` les plus anciennes (typiquement 24h).

### 6.3 RBAC

```
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries,verbs=get;list;watch;create;update;patch;delete
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/status,verbs=get;update;patch
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/finalizers,verbs=update
```

Pas de RBAC `secrets` en v1 (auth via `DefaultKeychain` qui lit le filesystem du Pod).

### 6.4 RBAC `ImageInventory` augmenté

```
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries,verbs=get;list;watch;create;update;patch;delete
```

Pour permettre à `ImageInventory` de gérer les CR enfants.

---

## 7. Readstore — `internal/readstore/image/`

### 7.1 Single writer = `ImageRegistry` controller

Le `ProjectImagesHandler` côté `ImageInventory` est **supprimé**. L'interface `ImageWriter` change :

```go
type ImageWriter interface {
    // ReplaceForNamespace remplace toutes les contributions du couple
    // (portalRef, host, namespace) par les views fournies.
    // Les contributions des autres namespaces aux mêmes (registry, repo, tag)
    // sont préservées.
    ReplaceForNamespace(ctx context.Context, portalRef, host, namespace string, views []ImageView) error

    // RemoveForNamespace : appelé par le finalizer du CR (suppression).
    RemoveForNamespace(ctx context.Context, portalRef, host, namespace string) error
}
```

### 7.2 Dedup interne

Le store maintient :

```go
type store struct {
    // contributions[scope][entryKey] = ImageView (per-namespace contribution)
    contributions map[scopeKey]map[entryKey]ImageView
    // aggregated[portalRef][entryKey] = ImageView agrégée (workloads merged)
    aggregated map[string]map[entryKey]ImageView
    mu sync.RWMutex
}

type scopeKey struct{ Portal, Host, Namespace string }
type entryKey struct{ Registry, Repo, Tag string }
```

À chaque `ReplaceForNamespace` : recalcule l'aggregation pour les `entryKey` impactées (union des workloads, dernière `LatestVersion` non vide, etc.).

### 7.3 Reader

`ImageReader.ListByPortal(portalRef)` retourne les `ImageView` agrégées (namespaces visibles dans `Workloads[].Namespace`).

### 7.4 `ImageView` étendu

```go
type ImageView struct {
    PortalRef        string
    Registry         string
    Repository       string
    Tag              string
    TagType          TagType
    OriginalImage    string  // vide si injected
    MutatedImage     string
    ChangeType       string  // none|mutated|injected
    LatestVersion    string
    LatestCheckedAt  *time.Time
    LatestError      string
    UpgradeAvailable bool
    Workloads        []WorkloadRef
}
```

---

## 8. Métriques — `internal/metrics/metrics.go`

```go
ImageRegistryEntriesTotal   = NewGaugeVec(...)   // labels: portal, host, namespace
ImageRegistryUpgradesTotal  = NewGaugeVec(...)   // labels: portal, host, namespace
ImageRegistryMutatedTotal   = NewGaugeVec(...)   // labels: portal, host, namespace
ImageRegistryInjectedTotal  = NewGaugeVec(...)   // labels: portal, host, namespace
RegistryLookupTotal         = NewCounterVec(...) // labels: host, result (success|rate_limited|error|skipped)
RegistryLookupDuration      = NewHistogramVec(...) // label: host
RegistrySyncTotal           = NewCounterVec(...) // labels: result (success|error)
```

Cleanup dans le finalizer du CR.

---

## 9. Connect API — `proto/sreportal/v1/`

**Pas de nouveau service.** La CRD `ImageRegistry` étant controller-managed (artefact interne), il n'y a pas de besoin user de la manipuler via gRPC. Tout passe par `ImageService` enrichi.

Le message `Image` existant (`image.proto`) gagne :
- `latest_version` (string)
- `latest_checked_at` (Timestamp)
- `latest_error` (string)
- `upgrade_available` (bool)
- `change_type` (enum: `CHANGE_TYPE_NONE`, `CHANGE_TYPE_MUTATED`, `CHANGE_TYPE_INJECTED`)
- `original_image` (string, vide si `change_type=INJECTED`)

`ImageService.ListImages` retourne ces champs enrichis pour la page Images. Le filtrage côté serveur reste optionnel (la table UI peut filtrer côté client) ; ajouter `ListImagesRequest.filter` (par host / change_type / upgrade_available) si la liste devient grosse.

---

## 10. Portals remote (`IsRemote=true`)

- **`ImageInventory.SyncRegistryCRs` skip** entièrement pour les portals remote.
- Le `RemoteImage` proto (mirror du portal source) transporte déjà `latest_version` + champs associés (à étendre dans `internal/remoteclient/` + proto remote sync).
- Côté shadow, `FetchRemoteImagesHandler` projette les `RemoteImage` reçus directement dans le readstore via `ReplaceForNamespace` (en utilisant le même writer que `ImageRegistry`).
- Conséquence : l'UI / API du shadow voit les `LatestVersion` calculés côté source, sans aucun appel registry local.

---

## 11. Web UI — extension de `web/src/features/image/`

**Pas de nouvelle feature module.** La CRD `ImageRegistry` étant interne, la page Images existante absorbe toute la nouvelle information. Pas de nouvel item sidebar.

### 11.1 Colonnes ajoutées

| Colonne | Source | Affichage |
|---|---|---|
| `Latest version` | `Image.latest_version` | Texte ; vide si `tag_type ≠ semver` |
| `Change type` | `Image.change_type` | Badge : `none` (gris), `mutated` (orange), `injected` (bleu) |
| `Last checked` | `Image.latest_checked_at` | Timestamp relatif (ex. "2h ago") |

Colonnes existantes conservées : `Registry` (host), `Repository`, `Tag` (= `original_tag`), `Workloads` (count + drill-down).

### 11.2 Détail par row

Hover ou row-expand révèle :
- `OriginalImage` (vide si `injected`)
- `MutatedImage`
- `LatestError` (si non vide)
- Liste complète des workloads avec `Kind / Namespace / Name / Container`.

### 11.3 Filtres / facets

- **Host** (registry) — multi-select.
- **Namespace** — multi-select.
- **Change type** — `none` / `mutated` / `injected`.
- **Upgrade available** — toggle.
- **Tag type** — `semver` / `commit` / `digest` / `latest` / `other`.

### 11.4 Group-by optionnel

Toggle "group by host" : insère des header rows avec compteurs (total images / upgrades / mutated / injected) par host. Ça reproduit la "vue registries" sans page dédiée.

### 11.5 Highlight upgrades

Les rows avec `upgrade_available=true` ont un fond légèrement coloré + une icône d'alerte douce. Filtre rapide "upgrades only" épingle l'usage le plus fréquent.

---

## 12. MCP — extension de `internal/mcp/image_server.go`

**Pas de nouveau serveur MCP.** Les nouveaux tools sont ajoutés au serveur existant `/mcp/image`.

Tools ajoutés :

- `list_upgrades(portal, host?)` → entries où `UpgradeAvailable=true`. Filtre optionnel par host.
- `list_mutations(portal, host?, change_type?)` → entries où `ChangeType ∈ {mutated, injected}`. Filtre optionnel par host et type.
- `summary(portal)` → counts agrégés par host (`images`, `upgrades`, `mutated`, `injected`) — équivalent du group-by UI en JSON.

Tools existants (ex. `list_images`) retournent les champs enrichis du proto `Image` sans modification d'interface.

---

## 13. Câblage `cmd/main.go`

```go
hostLimiter      := registry.NewHostLimiter()
registryAdapter  := registry.NewCraneClient()

// ImageInventory reconciler : nouveau handler SyncRegistryCRs
ii := imageinventoryctrl.NewImageInventoryReconciler(
    mgr.GetClient(),
    /* readstore writer NOT injected anymore */
)
ii.SetupWithManager(mgr)

// ImageRegistry reconciler
ir := imageregistryctrl.NewImageRegistryReconciler(
    mgr.GetClient(),
    imageStore,           // ImageReader + ImageWriter
    registryAdapter,
    hostLimiter,
)
ir.SetupWithManager(mgr)
```

Indexer `spec.portalRef` ajouté pour `ImageRegistry`.

---

## 14. Tests

### 14.1 Unitaires

| Fichier | Couverture |
|---|---|
| `internal/domain/imageregistry/version_test.go` | `PickLatestSemver` table-driven : versions stables/RC, préfixe `v`, ignore `latest`. |
| `internal/domain/imageregistry/aggregator_test.go` | `none`/`mutated`/`injected`, init containers, dédup multi-workload, group par `(host, ns)`. |
| `internal/domain/imageregistry/naming_test.go` | Hash stable, longueur 12, charset RFC1123. |
| `internal/registry/crane_client_test.go` | `httptest` `/v2/<repo>/tags/list` : succès, 401, 404, 429 → `ErrRateLimited`, 5xx. |
| `internal/registry/rate_limiter_test.go` | Wait sérialise ; limites hardcodées par host ; default rate. |
| `internal/readstore/image/store_test.go` | `ReplaceForNamespace` agrège correctement, `RemoveForNamespace` retire les contributions, dédup global. |
| `internal/controller/imageregistry/chain/select_due_images_test.go` | Catch-up jitter actif si > 50% due, normal sinon ; nouveau CR (Status vide) traite tout immédiatement. |
| `internal/controller/imageregistry/chain/resolve_latest_versions_test.go` | Lookup origin pour `mutated/none`, lookup mutated pour `injected`, erreurs propagées dans `LastError`. |
| `internal/controller/imageregistry/chain/update_readstore_test.go` | Vues construites avec champs `Latest*`, écrites via `ReplaceForNamespace`. |
| `internal/controller/imageinventory/chain/sync_registry_crs_test.go` | Création/patch/suppression des CR enfants, `IsRemote=true` skip, ownerRef set, hash naming. |
| `internal/webhook/v1alpha1/imageregistry_webhook_test.go` | Validation `OriginalImage` selon `ChangeType`. |

### 14.2 Envtest

`internal/controller/imageregistry/suite_test.go` :
- Crée Portal + ImageInventory + scénarios pré-remplis (Pods + workloads avec/sans mutation/injection).
- Vérifie : CR enfants créés avec hash stable, `Spec.Images` cohérent, `Status.Images` rempli après reconcile, `LastCheckedAt` avancé.
- Cas `injected` : `OriginalImage=""`, lookup latest sur `MutatedImage`.
- Cas `IsRemote=true` : aucun CR `ImageRegistry` créé, readstore peuplé via projection remote.
- Cas finalizer : delete CR → readstore nettoyé + métriques nettoyées.
- Cas redémarrage : recreate manager avec Status pré-rempli, vérifier que seules les images "due" sont retraitées.

### 14.3 Coverage

Cible 80%+ globale. 90% sur `domain/imageregistry` (logique pure).

---

## 15. Ordre de build

1. **Domaine** : `naming.go`, `aggregator.go`, `version.go`, ports + tests unitaires.
2. **Adapter registry** : `crane_client.go`, `rate_limiter.go` + tests.
3. **Types CRD** : `kubebuilder create api/webhook` ImageRegistry, edit types, `make manifests generate helm doc`.
4. **Webhook validation** + tests.
5. **Readstore** : refactor `ImageWriter` interface, `ReplaceForNamespace` / `RemoveForNamespace` + tests.
6. **Étendre `ImageInventory`** : status `Registries[]`, handler `SyncRegistryCRs`, suppression de `ProjectImagesHandler` + tests + envtest.
7. **Controller `ImageRegistry`** : chain handlers + reconciler + finalizer + tests + envtest.
8. **Câblage `cmd/main.go`** + indexer.
9. **Métriques** + cleanup finalizer.
10. **Proto** : étendre `Image` message (`latest_version`, `latest_checked_at`, `latest_error`, `upgrade_available`, `change_type`, `original_image`), `make proto`, mapping `ImageService.ListImages` enrichi.
11. **Remote portals** : étendre `RemoteImage` proto + projection shadow.
12. **MCP** : étendre `/mcp/image` avec `list_upgrades`, `list_mutations`, `summary`.
13. **Web UI** : extension de `web/src/features/image/` — colonnes `Latest version` / `Change type` / `Last checked`, filtres, row-expand, group-by host.
14. **Doc** : README + exemples YAML.

---

## 16. Cas limites & risques

- **Hash collision** : 12 chars hex = 48 bits. Prob. collision pour 1000 entrées ≈ 1.8e-12. Acceptable. Si jamais : la création K8s échoue (409 Conflict sur le nom), `SyncRegistryCRs` log + skip + métrique d'erreur.
- **Namespace > 63 chars** : impossible côté K8s (limite RFC 1123). Donc le hash 12 chars est toujours valide.
- **Catch-up storm après long downtime** : token bucket plafonne le débit registry (ex. 5 req/min sur docker.io = 1000 images en ~3h30). Le jitter §6.2 évite la file en mémoire.
- **Étalement du catch-up jitter** : si 1000 images due au boot, jitter aléatoire `[0, 24h]` les répartit ≈ 42 images/h en moyenne, bien sous la limite token bucket. Cohérent.
- **Status etcd 1.5 MB** : avec découpage par `(portal, host, ns)`, chaque CR fait typiquement < 50 entries × ~600B = 30 KB. Largement OK.
- **Portal supprimé** : `ImageInventory` est cleanup en cascade par owner ref (Portal → Inventory). Les `ImageRegistry` partent aussi (owner ref Inventory → Registry).
- **Webhook ne supporte pas la cross-ns mutation** : si un Pod du ns A est muté avec une image originairement déclarée dans le ns B (cas tordu), on attribue au ns A (Pod observé). Acceptable.

---

## 17. Décisions actées

| # | Décision | Rationale |
|---|---|---|
| 1 | CR par `(portal, host, namespace)`, pas global | Lecture allégée à 1000+ images cluster-wide |
| 2 | Auto-création par `ImageInventory` (pas de CR `imageregistrydiscovery` séparé) | Évite duplication, ImageInventory est déjà la source de vérité |
| 3 | Pas de troncature `Status.Images` | Le découpage par ns borne naturellement la taille |
| 4 | Pas de cap `Workloads` par entry | Borné naturellement par taille du namespace |
| 5 | Hash 12-char sha256 hex comme nom de CR (sans préfixe) | Évite limite 63 chars, prédictible |
| 6 | `ImageInventory.Status.Registries[]` lookup hash → `(host, ns)` | Debug & traçabilité |
| 7 | `ChangeType ∈ {none, mutated, injected}` | Couvre les 3 cas observés (webhook rewrite, sidecar inject) |
| 8 | `OriginalImage=""` pour `injected` ; lookup latest se fait sur `MutatedImage` | Sémantique claire, cas (a) du fallback uniquement |
| 9 | Pas de fallback runtime sur erreur de lookup | Cas (b) abandonné — simplicité, erreur visible dans Status |
| 10 | CR non user-managed en v1 (pas d'auth/allowlist/semver/interval user) | Auth via `DefaultKeychain`, pas de cas justifiant la surface user pour l'instant |
| 11 | OwnerRef = `ImageInventory` | GC automatique cohérent avec la chaîne de dépendances |
| 12 | Purge du CR si host disparaît | Sans config user à préserver, pas de risque |
| 13 | Reconcile 1×/jour (24h hardcoded) | Réduit le coût registry, suffit pour des semver releases |
| 14 | Granularité due-or-not par image (pas par CR) | Étalement naturel des appels |
| 15 | Catch-up jitter au démarrage (β) si > 50% due | Évite la file mémoire géante après long downtime |
| 16 | Token bucket per host, limites hardcodées | Simple v1, ajustable plus tard via config si besoin |
| 17 | Pas de cache de tags | Reconcile 1×/jour + bucket = pression suffisante |
| 18 | Single writer readstore = controller `ImageRegistry` | Élimine l'ambiguïté multi-écrivain |
| 19 | Dedup global readstore par `(portal, registry, repo, tag)`, ns en info | Reflète la réalité (image identique en plusieurs ns = même version) |
| 20 | Portals remote : `ImageInventory` skip création CR ; latest projeté via `RemoteImage` étendu | Évite les appels registry redondants côté shadow |
| 21 | Conditions minimales : `Ready` only | Détail dans `LastError` + métriques Prometheus |
| 22 | Pas de pages UI dédiées `Registries` ; tout sur la page Images existante | CRD `ImageRegistry` est interne (controller-managed), pas une entité de premier ordre pour l'user |
| 23 | Pas de service gRPC `ImageRegistryService` ; extension de `ImageService.ListImages` | Cohérent avec §22, une seule API |
| 24 | Pas de serveur MCP dédié ; extension de `/mcp/image` existant | Cohérent avec §22, un seul endpoint MCP |
