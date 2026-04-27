# Image Scanner — Passage en event-driven incrémental

**Date :** 2026-04-21  
**Branche cible :** feat/image-inventory-crd  
**Statut :** approuvé, en attente d'implémentation

## Contexte

Le `Scanner` actuel est un `manager.Runnable` qui tourne en boucle toutes les `interval` (défaut 5 min), liste tous les `ImageInventory` CRs, et rescanne la totalité des workloads correspondants à chaque tick. Ce modèle est coûteux : si rien ne change dans le cluster, tous les workloads sont relus pour rien.

L'objectif est de remplacer ce polling global par un controller event-driven qui réagit aux changements des workloads Kubernetes et ne scanne que l'objet modifié.

## Contraintes

- Mise à jour **incrémentale** : seul le workload modifié est scanné sur un événement.
- Scan complet **acceptable** uniquement sur create ou changement de spec d'un `ImageInventory` (événement rare sur le CR lui-même).
- Le scan complet doit être **thread-safe**.
- Le champ `spec.interval` conserve sa sémantique : resync périodique de sécurité en cas d'événement manqué.

---

## Architecture

### Ce qui disparaît

- `internal/controller/image/scanner.go` — le `manager.Runnable` et son ticker sont supprimés.

### Ce qui change

| Composant | Changement |
|---|---|
| `internal/readstore/image/store.go` | Modèle interne `map[portalRef]map[WorkloadKey][]ImageView`, nouvelle interface `ImageWriter` |
| `ImageInventoryReconciler` | Nouveau handler `ScanWorkloadsHandler` + `DeletePortal` sur suppression + `RequeueAfter: interval` |
| `cmd/main.go` | Retirer `NewScanner`, ajouter `WorkloadHandler` + `SetupWorkloadReconcilersWithManager` |

### Ce qui est ajouté

| Composant | Rôle |
|---|---|
| `internal/domain/image/workload_key.go` | Type `WorkloadKey{Kind, Namespace, Name}` |
| `internal/controller/image/handler.go` | `WorkloadHandler` — logique partagée upsert/delete |
| `internal/controller/image/workload_reconcilers.go` | 5 thin reconcilers (Deployment, StatefulSet, DaemonSet, CronJob, Job) |
| `internal/controller/imageinventory/chain/scan_workloads.go` | `ScanWorkloadsHandler` |

---

## Flux d'événements

```
Workload modifié (ex: Deployment)
    → DeploymentImageReconciler.Reconcile
        → WorkloadHandler.HandleUpsert(ctx, wk, podSpec, objLabels)
            → client.List tous les ImageInventory CRs
            → filtre en mémoire (namespaceFilter, labelSelector, watchedKinds)
            → pour chaque match : store.ReplaceWorkload(portalRef, wk, images)

Workload supprimé
    → DeploymentImageReconciler.Reconcile (NotFound)
        → WorkloadHandler.HandleDelete(ctx, wk)
            → store.DeleteWorkloadAllPortals(wk)

ImageInventory créé / spec modifié
    → ImageInventoryReconciler chain
        → ValidateSpecHandler
        → ValidatePortalRefHandler
        → ScanWorkloadsHandler : scan complet → store.ReplaceAll(portalRef, byWorkload)
        → UpdateStatusHandler : RequeueAfter = inv.Spec.EffectiveInterval()

ImageInventory supprimé
    → ImageInventoryReconciler
        → store.DeletePortal(portalRef)
```

---

## Section 1 — Refactor du store

### Type `WorkloadKey` (domaine)

Fichier : `internal/domain/image/workload_key.go`

```go
type WorkloadKey struct {
    Kind      string
    Namespace string
    Name      string
}
```

### Interface `ImageWriter` (mise à jour)

```go
type ImageWriter interface {
    // ReplaceWorkload met à jour la contribution d'un seul workload pour un portal.
    ReplaceWorkload(ctx context.Context, portalRef string, wk WorkloadKey, images []ImageView) error

    // DeleteWorkloadAllPortals supprime la contribution d'un workload dans tous les portals.
    DeleteWorkloadAllPortals(ctx context.Context, wk WorkloadKey) error

    // ReplaceAll remplace atomiquement toute la projection d'un portal (scan complet).
    ReplaceAll(ctx context.Context, portalRef string, byWorkload map[WorkloadKey][]ImageView) error

    // DeletePortal supprime toutes les projections d'un portal.
    DeletePortal(ctx context.Context, portalRef string) error
}
```

L'ancienne méthode `Replace(portalRef, []ImageView)` est supprimée.

### Modèle interne du store

```go
type Store struct {
    mu   sync.RWMutex
    data map[string]map[WorkloadKey][]ImageView  // [portalRef][workloadKey]
    // canal broadcast inchangé
}
```

**Thread-safety :**
- Lectures (`List`, `Count`) : `RLock`
- Écritures : `Lock` complet
- `ReplaceAll` : un seul `Lock`, remplace `data[portalRef]` atomiquement — aucun lecteur ne voit un état partiel

**Agrégation à la lecture :** `List` aplatit toutes les contributions `data[portalRef]` puis applique la déduplication existante par `(registry, repository, tag)`.

---

## Section 2 — WorkloadHandler et thin reconcilers

### `WorkloadHandler`

Fichier : `internal/controller/image/handler.go`

```go
type WorkloadHandler struct {
    client client.Client
    store  domainimage.ImageWriter
}

func (h *WorkloadHandler) HandleUpsert(
    ctx context.Context,
    wk domainimage.WorkloadKey,
    spec corev1.PodSpec,
    objLabels labels.Set,
) error

func (h *WorkloadHandler) HandleDelete(ctx context.Context, wk domainimage.WorkloadKey) error
```

**`HandleUpsert` :**
1. `client.List` tous les `ImageInventory` (tous namespaces)
2. Filtre en mémoire par `namespaceFilter`, `labelSelector`, `watchedKinds`
3. Pour chaque match : extraire images du `PodSpec` → `store.ReplaceWorkload`
4. Retourner la première erreur de store (les autres portals sont quand même mis à jour)

**`HandleDelete` :**
1. `store.DeleteWorkloadAllPortals(wk)`

**Filtre `labelSelector` :** parsé via `labels.Parse` (déjà validé côté CR, donc parsing infaillible). En cas d'échec inattendu : fail-open (traiter comme "pas de filtre"), logguer.

### 5 thin reconcilers

Fichier : `internal/controller/image/workload_reconcilers.go`

```go
type DeploymentImageReconciler  struct { handler *WorkloadHandler }
type StatefulSetImageReconciler struct { handler *WorkloadHandler }
type DaemonSetImageReconciler   struct { handler *WorkloadHandler }
type CronJobImageReconciler     struct { handler *WorkloadHandler }
type JobImageReconciler         struct { handler *WorkloadHandler }
```

Patron commun (exemple Deployment) :

```go
func (r *DeploymentImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var obj appsv1.Deployment
    if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
        if apierrors.IsNotFound(err) {
            wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: req.Namespace, Name: req.Name}
            return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
        }
        return ctrl.Result{}, fmt.Errorf("get deployment: %w", err)
    }
    wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: obj.Namespace, Name: obj.Name}
    return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}
```

`CronJobImageReconciler` passe `obj.Spec.JobTemplate.Spec.Template.Spec`.

**Enregistrement :**

```go
func SetupWorkloadReconcilersWithManager(mgr ctrl.Manager, h *WorkloadHandler) error
```

Appelle `ctrl.NewControllerManagedBy(mgr).For(&appsv1.Deployment{}).Named("deployment-image").Complete(...)` pour chaque kind.

**RBAC :** chaque thin reconciler porte ses propres markers kubebuilder :

```go
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
```

Ces markers déclenchent la regénération du `ClusterRole` via `make manifests`.

---

## Section 3 — Changements de l'`ImageInventoryReconciler`

### Chaîne de handlers mise à jour

```
ValidateSpecHandler → ValidatePortalRefHandler → ScanWorkloadsHandler → UpdateStatusHandler
```

### `ScanWorkloadsHandler`

Fichier : `internal/controller/imageinventory/chain/scan_workloads.go`

```go
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[...]) error {
    inv := rc.Resource
    if inv.Status.ObservedGeneration == inv.GetGeneration() && !rc.Data.ForceRescan {
        return nil
    }
    byWorkload, err := h.scanAll(ctx, inv)
    if err != nil {
        return fmt.Errorf("full scan: %w", err)
    }
    return h.store.ReplaceAll(ctx, inv.Spec.PortalRef, byWorkload)
}
```

`scanAll` réutilise la logique de `scanner.scanInventory` : liste les workloads par kind avec `namespaceFilter` et `labelSelector`, construit `map[WorkloadKey][]ImageView`.

En cas d'erreur partielle (un kind échoue) : retourner l'erreur sans appeler `ReplaceAll` — l'ancien état reste dans le store.

### `ChainData`

```go
type ChainData struct{}  // réservé pour usage futur
```

### Resync périodique

`UpdateStatusHandler` pose `rc.Result.RequeueAfter = inv.Spec.EffectiveInterval()` en fin de reconcile réussi. `ScanWorkloadsHandler` s'exécute à chaque passage de la chaîne (spec valide + portal trouvé), que ce soit suite à un changement de spec ou à un requeue périodique. La fréquence maximale est donc bornée par `interval`. Aucun mécanisme `ForceRescan` n'est nécessaire — controller-runtime ne permet pas de distinguer un requeue d'un event, et l'uniformité est préférable à la complexité.

### Suppression

Dans `Reconcile`, avant la chaîne :

```go
if !inv.DeletionTimestamp.IsZero() {
    return ctrl.Result{}, r.store.DeletePortal(ctx, inv.Spec.PortalRef)
}
```

Pas de finalizer — le store est en mémoire, le scan complet au démarrage (via requeue initial) recouvre toute perte.

---

## Section 4 — Gestion des erreurs

| Situation | Comportement |
|---|---|
| API server indisponible dans `HandleUpsert` (listing) | Retourner erreur → requeue auto |
| `store.ReplaceWorkload` échoue pour un portal | Logguer, continuer les autres portals, retourner première erreur |
| `scanAll` échoue partiellement | Retourner erreur, ne pas appeler `ReplaceAll`, conserver ancien état |
| `labelSelector` parsing inattendu dans `HandleUpsert` | Fail-open (pas de filtre), logguer |
| Concurrence `HandleUpsert` ↔ `ScanWorkloadsHandler` | Acceptable — store thread-safe, convergence au prochain event/resync |

---

## Section 5 — Tests

### Store (`internal/readstore/image/`)

- `ReplaceWorkload` + `List` : agrégation et déduplication multi-workloads
- `DeleteWorkloadAllPortals` : suppression cross-portals
- `ReplaceAll` : atomicité (lecteur concurrent ne voit pas d'état partiel)
- `DeletePortal` : suppression complète du portal

### `WorkloadHandler` (`internal/controller/image/`)

- `HandleUpsert` : plusieurs `ImageInventory` matchants/non-matchants → seuls les bons portals mis à jour
- `HandleDelete` : appel à `DeleteWorkloadAllPortals`
- Filtres : `namespaceFilter` exact/vide, `labelSelector` match/no-match, `watchedKinds` filtrage

Outillage : `client.NewFakeClient`, mock `ImageWriter` — pas d'envtest.

### Thin reconcilers

- NotFound → `HandleDelete` appelé
- Objet présent → `HandleUpsert` appelé avec bon `PodSpec`

### `ScanWorkloadsHandler`

- `Generation != ObservedGeneration` → `ReplaceAll` appelé
- `Generation == ObservedGeneration`, `ForceRescan = false` → scan skippé
- `ForceRescan = true` → `ReplaceAll` appelé
- Erreur partielle → `ReplaceAll` non appelé

Outillage : envtest (suite existante de l'`ImageInventoryReconciler`).

---

## Fichiers à créer / modifier

| Fichier | Action |
|---|---|
| `internal/domain/image/workload_key.go` | Créer |
| `internal/domain/image/writer.go` | Modifier (nouvelle interface) |
| `internal/readstore/image/store.go` | Modifier (nouveau modèle interne) |
| `internal/readstore/image/store_test.go` | Modifier |
| `internal/controller/image/handler.go` | Créer |
| `internal/controller/image/workload_reconcilers.go` | Créer |
| `internal/controller/image/scanner.go` | Supprimer |
| `internal/controller/image/scanner_test.go` | Supprimer |
| `internal/controller/imageinventory/chain/handlers.go` | Modifier (ChainData, UpdateStatusHandler) |
| `internal/controller/imageinventory/chain/scan_workloads.go` | Créer |
| `internal/controller/imageinventory/imageinventory_controller.go` | Modifier (DeletePortal, ForceRescan) |
| `cmd/main.go` | Modifier (retirer Scanner, ajouter WorkloadHandler) |
