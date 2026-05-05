# Plan — Registry version lookup + ImageRegistry CRD

Statut : draft / à valider

Ce document couvre deux fonctions liées :

1. **F1** — Pour chaque image trouvée dans le cluster par le controller `ImageInventory`, interroger la registry pour récupérer la dernière version semver disponible (hors `latest`).
2. **F2** — Nouvelle CRD `ImageRegistry` qui agrège, par registry et par portal, l'ensemble des images trouvées dans le cluster avec, pour chaque entrée : image d'origine (template PodSpec), image mutée (Pod observé), dernière version disponible sur la registry d'origine.

F2 absorbe F1 : la résolution registry vit dans le controller `ImageRegistry`, pas dans `ImageInventory`.

---

## 1. Exploration de l'existant

### 1.1 Controller `ImageInventory`

`internal/controller/imageinventory/`

Chain :

```
ValidateSpec → ValidatePortalRef → FetchRemoteImages (remote only) → ScanWorkloads (local only) → ProjectImages → UpdateStatus
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
  Pas de champ « dernière version disponible ».
- `TagType` ∈ `{semver, commit, digest, latest, other}` (parser dans `internal/domain/image/parser.go`).
- `WorkloadRef.Source` ∈ `{spec, pod}` distingue l'image template (avant MutatingWebhook) de celle observée dans le Pod (post-mutation).

### 1.2 Dépendances déjà présentes

- `github.com/google/go-containerregistry v0.21.5` — utilisé pour le parsing ; `pkg/v1/remote.List` permet `ListTags`, `authn.DefaultKeychain` pour l'auth (lit `~/.docker/config.json`, IRSA, GCR creds).
- `golang.org/x/mod/semver` — disponible transitivement dans go.sum (sinon ajout 1 ligne).

### 1.3 API & UI

- Connect API : `proto/sreportal/v1/image.proto` expose `Image{registry, repository, tag, tag_type, workloads}`. `RemoteImage` mirror dans `internal/remoteclient/`.
- Web UI : page Images existante dans `web/src/features/image/`.
- MCP server `internal/mcp/image_server.go` monté à `/mcp/image`.

### 1.4 Pattern de référence — Alertmanager

Toutes les CRDs du projet sont **namespaced** (`PROJECT` confirmé). `Alertmanager` est un bon modèle :

- `Spec` user-déclaré (URL, portalRef, IsRemote, TLS).
- `Status` rempli par contrôleur via fetch externe (alerts API) + cache TLS.
- Chain handlers : `validate → fetch → update_status`.
- Réutilise `RemoteTLSConfig` partagée avec Portal.
- Indexer field `spec.portalRef` (cf. `cmd/main.go`).

### 1.5 Aspects critiques à anticiper

- Scan inventaire toutes les `EffectiveInterval()` (5 min par défaut). **Une requête registry par image à chaque cycle = appels réseau coûteux + risque de rate-limit** (Docker Hub 100/6h anonyme). → Cache avec TTL long (1h+).
- Inventaires `IsRemote=true` : la résolution latest a déjà été faite côté portal source ; le shadow ne doit pas re-interroger.
- Tags `latest`, `digest`, `commit`, `other` ne sont pas comparables sémantiquement → résolution uniquement pour `TagTypeSemver`.
- Status CR limité par etcd à ~1.5 MB → plafonner `Status.Images` (proposé : `MaxImages=500` par défaut, tronquer + condition `Truncated`).

---

## 2. CRD `ImageRegistry`

### 2.1 Création

```bash
kubebuilder create api    --group sreportal --version v1alpha1 --kind ImageRegistry
kubebuilder create webhook --group sreportal --version v1alpha1 --kind ImageRegistry \
  --defaulting --programmatic-validation
```

Namespaced (cohérent avec tout le projet).

### 2.2 Spec — `api/v1alpha1/imageregistry_types.go`

```go
type ImageRegistrySpec struct {
    // host identifie la registry (ex. "docker.io", "ghcr.io", "registry.k8s.io").
    // C'est la registry d'origine telle que vue dans le PodSpec template.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Pattern=`^[a-z0-9.-]+(:[0-9]+)?$`
    Host string `json:"host"`

    // portalRef est le Portal dont cet inventaire registry est dérivé.
    // +kubebuilder:validation:Required
    PortalRef string `json:"portalRef"`

    // interval contrôle la fréquence de rafraîchissement des "latest versions".
    // Défaut: 1h. Découplé d'ImageInventory pour limiter les appels registry.
    // +optional
    Interval metav1.Duration `json:"interval,omitempty"`

    // auth configure l'authentification à la registry.
    // +optional
    Auth *ImageRegistryAuth `json:"auth,omitempty"`

    // tls configure TLS (insecureSkipVerify, caSecretRef) — réutilise RemoteTLSConfig.
    // +optional
    TLS *RemoteTLSConfig `json:"tls,omitempty"`

    // repositoryAllowlist optionnellement restreint les repositories pris en compte.
    // Patterns glob (* et ?). Vide = tout.
    // +optional
    RepositoryAllowlist []string `json:"repositoryAllowlist,omitempty"`

    // semverConstraint optionnel (ex. "<2.0.0", "~1.25") pour ne retenir comme
    // "latest" que les versions satisfaisant la contrainte. Vide = pas de contrainte.
    // +optional
    SemverConstraint string `json:"semverConstraint,omitempty"`

    // maxImages plafond de sécurité sur le nombre d'entrées dans Status (défaut 500).
    // Au-delà, le contrôleur tronque et émet une condition Warning.
    // +optional
    MaxImages int32 `json:"maxImages,omitempty"`
}

type ImageRegistryAuth struct {
    // pullSecretRef pointe sur un Secret de type kubernetes.io/dockerconfigjson.
    // +optional
    PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}
```

### 2.3 Status

```go
type ImageRegistryStatus struct {
    ObservedGeneration int64        `json:"observedGeneration,omitempty"`
    LastScanTime       *metav1.Time `json:"lastScanTime,omitempty"`
    LastError          string       `json:"lastError,omitempty"`

    // imageCount nombre d'images uniques (registry, repo, originalTag).
    ImageCount int32 `json:"imageCount,omitempty"`

    // upgradeAvailableCount nombre d'entrées dont latestVersion > tag courant.
    UpgradeAvailableCount int32 `json:"upgradeAvailableCount,omitempty"`

    // mutatedCount nombre d'entrées dont originalImage != mutatedImage.
    MutatedCount int32 `json:"mutatedCount,omitempty"`

    // images : liste enregistrée. Plafonnée par Spec.MaxImages.
    // +listType=map
    // +listMapKey=key
    // +optional
    Images []ImageRegistryEntry `json:"images,omitempty"`

    // truncated vrai si la liste Images a été tronquée.
    Truncated bool `json:"truncated,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ImageRegistryEntry struct {
    // key = sha256(repository|originalTag|mutatedRegistry|mutatedRepository|mutatedTag)
    // sert de clé stable pour listMapKey.
    // +kubebuilder:validation:Required
    Key string `json:"key"`

    // originalImage ref telle que déclarée dans le PodSpec template,
    // avant tout MutatingWebhook (registry/repo:tag).
    // +kubebuilder:validation:Required
    OriginalImage string `json:"originalImage"`

    // mutatedImage ref observée dans le Pod en cours d'exécution.
    // Égale à originalImage si aucune mutation n'a eu lieu.
    // +kubebuilder:validation:Required
    MutatedImage string `json:"mutatedImage"`

    // repository (ex. "library/nginx") — doublon parsing pour affichage.
    Repository string `json:"repository"`

    // originalTag tag déclaré dans le template.
    OriginalTag string `json:"originalTag"`

    // tagType classification (semver|commit|digest|latest|other).
    TagType string `json:"tagType"`

    // latestVersion plus haut tag semver disponible sur la registry d'origine
    // (hors "latest"). Vide si non applicable ou erreur.
    LatestVersion string `json:"latestVersion,omitempty"`

    // upgradeAvailable vrai si latestVersion existe et > originalTag.
    UpgradeAvailable bool `json:"upgradeAvailable,omitempty"`

    // latestCheckedAt date du dernier appel registry réussi.
    LatestCheckedAt *metav1.Time `json:"latestCheckedAt,omitempty"`

    // latestError dernière erreur de résolution (vide si succès).
    LatestError string `json:"latestError,omitempty"`

    // workloads liste les workloads qui utilisent cette image.
    Workloads []ImageRegistryWorkloadRef `json:"workloads,omitempty"`
}

type ImageRegistryWorkloadRef struct {
    Kind      string `json:"kind"`
    Namespace string `json:"namespace"`
    Name      string `json:"name"`
    Container string `json:"container"`
}
```

`printcolumns` : `Host`, `Portal`, `Images`, `Upgrades`, `Mutated`, `Last Scan`, `Age`.

### 2.4 Webhook validation — `internal/webhook/v1alpha1/imageregistry_webhook.go`

- Defaulting : `Interval=1h`, `MaxImages=500`.
- Validation : `Host` parsable via `name.NewRegistry(host)`, `SemverConstraint` parsable, patterns `RepositoryAllowlist` valides, `PortalRef` non vide.

---

## 3. Domaine — `internal/domain/imageregistry/`

### 3.1 Types

`types.go` : `Entry`, `WorkloadRef` — équivalents domaine sans tags Kubebuilder.

### 3.2 Aggregator

`aggregator.go` :

```go
func AggregateForRegistry(
    host string,
    byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView,
    allow []glob.Pattern,
) []Entry
```

Pour chaque `WorkloadKey` :

- Partitionne `views` en `specView` (un seul, `ContainerSource=spec`) et `podViews` (≥0).
- Pair par **container** : si un `podView` a le même `Container` que `specView`, c'est l'image mutée ; sinon mutated=original.
- Filtre : ne retient que les paires dont `specView.Registry == host` (la registry d'origine).
- Émet une `Entry` par paire unique `(repository, originalTag, mutatedRef)`.
- Workloads dédupliqués et triés.

### 3.3 Version

`version.go` (utilise `golang.org/x/mod/semver`) :

- `IsUpgradable(tt TagType) bool` → true uniquement pour `TagTypeSemver`.
- `PickLatestSemver(tags []string, constraint string) (string, error)` :
  - Filtre les tags semver.
  - Ignore `"latest"`.
  - Applique `constraint` si non vide.
  - Retourne le plus haut. Préserve le préfixe `v` si l'input l'avait.

### 3.4 Port registry

`registry_port.go` :

```go
type Client interface {
    ListTags(ctx context.Context, host, repository string, opts ClientOptions) ([]string, error)
}

type ClientOptions struct {
    Keychain       authn.Keychain // nil = anonyme
    InsecureTLS    bool
    RootCAs        *x509.CertPool
    RequestTimeout time.Duration
}
```

---

## 4. Adapter registry — `internal/registry/`

- `crane_client.go` : implémente `domainimageregistry.Client` via `pkg/v1/remote.List`. Construit `name.Repository`. Gère `404` (repo inexistant → tags vides + log). Gère `429` → erreur taggée `ErrRateLimited` pour requeue plus long.
- `keychain.go` : construit `authn.Keychain` à partir d'un Secret `dockerconfigjson` (lookup via `client.Reader`). Cache léger keyed par `(namespace, secretName, ResourceVersion)`.
- `cache.go` : TTL cache `(host, repo) → (tags, err, fetchedAt)`. TTL succès = `Spec.Interval`, TTL erreur = `min(Interval, 5m)`. Concurrency-safe.

---

## 5. Controller — `internal/controller/imageregistry/`

Chain pattern aligné sur Alertmanager :

```
ValidateSpec → ValidatePortalRef → CollectFromReadstore → ResolveLatestVersions → UpdateStatus
```

### 5.1 Handlers

**`chain/validate_spec.go`** : `Host` non vide, `PortalRef` non vide, sinon `ErrInvalidSpec`.

**`chain/validate_portal_ref.go`** : Get Portal `{ns/portalRef}`, sinon `ErrPortalNotFound`.

**`chain/collect_from_readstore.go`** :

- Étend `ImageReader` avec :
  ```go
  ListByWorkload(ctx, ImageFilters) (map[WorkloadKey][]ImageView, error)
  ```
- Appelle `domainimageregistry.AggregateForRegistry(host, byWorkload, allowlist)`.
- Stocke dans `ChainData.Entries`.

**`chain/resolve_latest_versions.go`** :

- Construit registry client (clé cache TLS+auth).
- Pour chaque `Entry` : lookup tags via cache → `PickLatestSemver(tags, constraint)`.
- Concurrence bornée (`errgroup` `SetLimit(8)`).
- Erreurs réseau non bloquantes : remplit `LatestError`, continue.
- `UpgradeAvailable = semver.Compare(latest, originalTag) > 0`.

**`chain/update_status.go`** :

- Tronque à `Spec.MaxImages` (tri : `UpgradeAvailable desc`, `Repository asc`, `OriginalTag asc`).
- Compute summary counts.
- Patch `Status.Conditions` (Ready, Truncated si applicable).
- Patch `LastScanTime`.

### 5.2 Reconciler

`imageregistry_controller.go` :

- Watche `ImageRegistry`.
- **Watche aussi le readstore** : controller souscrit à `domainimage.ImageReader.Subscribe()` ; quand le store change, enqueue les `ImageRegistry` du portal concerné (pattern `source.Channel`).
- `RequeueAfter: spec.EffectiveInterval()` à la fin pour rafraîchissement périodique.
- Finalizer : nettoie compteurs métriques.
- Field indexer `spec.portalRef`.

### 5.3 RBAC

```
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries,verbs=get;list;watch;create;update;patch;delete
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/status,verbs=get;update;patch
+kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/finalizers,verbs=update
+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch  // pull-secret + TLS
```

---

## 6. Métriques — `internal/metrics/metrics.go`

```go
ImageRegistryEntriesTotal   = NewGaugeVec(...)   // labels: registry, host, portal
ImageRegistryUpgradesTotal  = NewGaugeVec(...)   // labels: registry, host, portal
ImageRegistryMutatedTotal   = NewGaugeVec(...)   // labels: registry, host, portal
RegistryLookupTotal         = NewCounterVec(...) // labels: host, result (success|cache_hit|rate_limited|error)
RegistryLookupDuration      = NewHistogramVec(...) // label: host
RegistrySyncTotal           = NewCounterVec(...) // labels: registry, result
```

---

## 7. Remontée vers `ImageView` (page Images existante)

Pour que la page Images affiche `latestVersion` sans nouvel endpoint :

- Étendre `ImageView` avec `LatestVersion`, `LatestCheckedAt`, `LatestError`, `UpgradeAvailable`.
- Nouveau writer : `UpdateLatestVersion(portalRef, registry, repository, originalTag, info LatestInfo)`. Mute les `ImageView` matchants in-place sous lock, broadcast subscribe.
- `ResolveLatestVersionsHandler` (chain ImageRegistry) appelle ce writer après calcul.
- Single writer : ImageRegistry est l'unique producteur, pas de duplication d'état.

---

## 8. Connect API — `proto/sreportal/v1/`

Nouveau service `ImageRegistryService` (séparé d'`ImageService`) :

- `ListImageRegistries(portal_name)` → `[]ImageRegistry` (host, summary stats).
- `GetImageRegistry(portal_name, host)` → `ImageRegistry` avec `entries`.
- Mapping reflète `ImageRegistryEntry`.

`image.proto` existant : ajouter `latest_version`, `latest_checked_at`, `latest_error`, `upgrade_available` au message `Image` pour la page Images.

---

## 9. Web UI — `web/src/features/imageregistry/`

Nouvelle feature module :

- Page **Registries** : liste des `ImageRegistry` du portal courant (host, image count, badge "X upgrades available").
- Page **Registry detail** : table des entries avec colonnes `Original | Mutated | Tag | Latest | Workloads`. Mise en évidence quand `UpgradeAvailable=true`. Filtre `mutated only`.
- Sidebar : nouvel item « Registries » sous « Images ».

Page `Images` existante : ajouter colonne `Latest` (rendu possible par §7).

---

## 10. MCP — `internal/mcp/imageregistry_server.go`

Mount à `/mcp/imageregistry`. Tools :

- `list_registries(portal)` → JSON résumé.
- `list_upgrades(portal, registry)` → entries où `UpgradeAvailable=true`.
- `list_mutations(portal, registry)` → entries où `OriginalImage != MutatedImage`.

---

## 11. Câblage `cmd/main.go`

```go
registryAdapter  := registry.NewCraneClient()
keychainBuilder  := registry.NewKeychainBuilder(mgr.GetClient())
registryCache    := registry.NewCache()

ir := imageregistryctrl.NewImageRegistryReconciler(
    mgr.GetClient(),
    imageStore,            // ImageReader+Writer
    registryAdapter,
    keychainBuilder,
    registryCache,
)
ir.SetupWithManager(mgr)
```

Indexer `spec.portalRef` ajouté pour `ImageRegistry` (comme `ImageInventory`).

---

## 12. Tests

### 12.1 Unitaires

| Fichier | Couverture |
| --- | --- |
| `internal/domain/imageregistry/version_test.go` | `PickLatestSemver` table-driven : versions stables/RC, préfixe `v`, contrainte semver, ignore `latest`. |
| `internal/domain/imageregistry/aggregator_test.go` | Spec seule, spec+pod identiques, spec+pod différents, registry mismatch (skippé), allowlist glob, dédup multi-workload. |
| `internal/registry/crane_client_test.go` | `httptest` simulant `/v2/<repo>/tags/list` : succès, 401, 404, 429 → `ErrRateLimited`, 5xx. |
| `internal/registry/keychain_test.go` | Build keychain depuis Secret dockerconfigjson, fallback anonyme, secret introuvable. |
| `internal/registry/cache_test.go` | TTL succès/erreur, hit, expiry, accès concurrent. |
| `internal/controller/imageregistry/chain/*_test.go` | Un test par handler avec fakes (`fake.NewClientBuilder`, fake registry client), erreurs propagées, conditions mises. |
| `internal/webhook/v1alpha1/imageregistry_webhook_test.go` | Defaulting, validation host/semver/allowlist. |
| `internal/readstore/image/store_test.go` | Étendre pour `UpdateLatestVersion` (mute in-place, broadcast subscribe). |
| `internal/grpc/imageregistry_service_test.go` | List/Get retournent les CRDs filtrées par portal. |

### 12.2 Envtest — `internal/controller/imageregistry/suite_test.go`

- Crée Portal + ImageInventory pré-rempli (mock readstore) + ImageRegistry.
- Vérifie que `Status.Images` est rempli, `LastScanTime` avance, `Conditions[Ready]=True`.
- Cas finalizer : delete CR → métriques nettoyées.
- Cas `Truncated=true` quand `MaxImages` dépassé.
- Cas mutated vs original : deux ImageView (spec + pod) pour le même container → une seule entry avec `OriginalImage != MutatedImage`.

### 12.3 Intégration registry réelle

Hors CI (réseau requis). Script `scripts/test-registry-live.sh` optionnel : test marqué `// +build live` interrogeant `docker.io/library/nginx` réel. Documenté dans le README.

### 12.4 Coverage

Cible 80%+ globale (règle `CLAUDE.md`). 90% sur `domain/imageregistry` (logique pure).

---

## 13. Ordre de build

1. **Types & webhook** : `kubebuilder create api/webhook`, puis `make manifests generate helm doc`.
2. **Domaine** : `version.go`, `aggregator.go`, ports.
3. **Adapter registry** : `crane_client`, `keychain`, `cache` + tests.
4. **Readstore extensions** : `ListByWorkload`, `UpdateLatestVersion` + tests.
5. **Chain handlers** + tests unitaires.
6. **Controller + envtest suite**.
7. **Câblage `cmd/main.go`** + indexer.
8. **Métriques** + tests.
9. **Proto** : `make proto`, mapping gRPC, MCP server.
10. **`ImageView` extension** + propagation web UI Images.
11. **Web UI** nouvelle feature `imageregistry`.
12. **Doc** README + exemples YAML.

---

## 14. Décisions à valider avant de coder

1. **Granularité du CR** : un `ImageRegistry` par `(host, portal)` (proposé) ou un par `host` global cluster-wide ? Proposition suit le modèle Alertmanager.
2. **Auto-discovery** : faut-il qu'un opérateur découvreur crée auto les `ImageRegistry` manquants ? **Reco : non en v1** (user-declared seul) ; v2 = controller `imageregistrydiscovery` séparé.
3. **Limite Status** : `MaxImages=500` par défaut, troncature + condition Warning. Acceptable, ou mode « off-load » (Status léger + readstore seul) ? **Reco : v1 = troncature avec warning**, suffit pour 95% des clusters.
4. **Auth registry** : v1 = pull-secret + TLS (90% des cas). v2 = workload identity (IRSA, GKE WI).
5. **Compat `ImageInventory` actuel** : la résolution latest sort entièrement du chain inventory (pas de `ResolveLatestVersionsHandler` côté inventory). Inventory reste pure observation cluster. Alternative : garder un handler dans inventory pour avoir latest dispo sans CR ImageRegistry. **Reco : sortir** — plus DDD-propre, créer un `ImageRegistry` est trivial.
