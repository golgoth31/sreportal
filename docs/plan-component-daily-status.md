# Plan : statut journalier (30 jours glissants) + requeue après minuit

## Contexte

Le CR `Component` expose déjà `status.computedStatus` (maintenance, incidents, déclaré). On veut **persister dans le statut** le **pire état observé par jour calendaire** sur une fenêtre **glissante de 30 jours (UTC)**, en réutilisant la hiérarchie de sévérité existante (`internal/domain/component` — `StatusSeverityRank`).

Sans réconciliation dédiée, un composant peu sollicité pourrait **ne pas passer le cap de minuit** : l’entrée du **nouveau jour** ne serait pas initialisée tant qu’aucun autre événement ne déclenche un reconcile. D’où un **`RequeueAfter` calculé jusqu’à la prochaine minuit UTC** pour **réconcilier une fois par jour** et **initialiser le statut du jour courant** avec l’état effectif actuel (puis la logique habituelle de merge « pire de la journée » s’applique).

---

## Objectifs

1. **Historique** : au plus 30 entrées `{ date UTC YYYY-MM-DD, worstStatus }`, sans doublon de date, ordre cohérent (ex. croissant).
2. **Merge** : à chaque reconcile réussi, mettre à jour le jour courant avec `max(severity(worst_existant), severity(computedStatus))`.
3. **Pruning** : retirer les jours hors fenêtre (jour courant UTC + 29 jours précédents, ou règle équivalente documentée).
4. **Minuit UTC** : après chaque reconcile réussi, fixer `ctrl.Result.RequeueAfter = duration jusqu’à la prochaine minuit UTC` pour forcer un reconcile au début du jour suivant et **poser la base du jour** avec l’état courant (premier échantillon du jour = état à minuit si le reconcile tombe juste après minuit, ou dès le premier passage dans la journée sinon).

---

## API (`api/v1alpha1/component_types.go`)

- Introduire un type entrée journalière, par exemple :

  - `date` : `string` au format **YYYY-MM-DD** (UTC).
  - `worstStatus` : aligné sur `ComputedComponentStatus`.

- Ajouter sur `ComponentStatus` un champ du type **slice** (ex. `dailyWorstStatus`), `+optional`, avec `+listType=map` et `+listMapKey=date` (cohérent avec `conditions`, supporte le strategic merge patch).

- Après modification : **`make helm`** puis **`make doc`** (ne pas éditer les YAML CRD à la main).

---

## Domaine (`internal/domain/component/`)

- Type valeur : **`DailyStatus`** `{ Date string /* YYYY-MM-DD */, WorstStatus ComponentStatus }`.
- Ajouter `DailyStatus` dans `ComponentView` (slice `[]DailyStatus`).

- Fonctions pures testables, sans dépendance K8s :

  - `MergeDailyWorst(history []DailyStatus, today string, current ComponentStatus, windowDays int) []DailyStatus` — merge + prune sur N jours (30), retourne la slice mise à jour (tri croissant par date) ;
  - `DurationUntilNextMidnightUTC(now time.Time) time.Duration` (borne minimale > 0, ex. 1s, si horloge pile sur minuit — éviter zéro).

- Réutiliser **`StatusSeverityRank`** pour le « pire ».

---

## Chaîne controller (`internal/controller/component/chain/`)

### Nouveau handler : `MergeDailyStatusHandler` (entre `ComputeStatus` et `UpdateStatus`)

- Handler dédié (SRP) inséré **après** `ComputeStatusHandler` et **avant** `UpdateStatusHandler`.
- Responsabilité unique : appeler la logique domaine `MergeDailyWorst` sur `comp.Status.DailyWorstStatus` avec `rc.Data.ComputedStatus` et la date UTC courante, puis pruning 30 jours.
- Ne pose **pas** de `RequeueAfter` (pas le dernier handler).

### Requeue minuit dans `UpdateStatusHandler` (dernier handler)

- Après mise à jour réussie et projection ReadStore :

  - calculer `d := DurationUntilNextMidnightUTC(time.Now().UTC())` (ou horloge injectée en test) ;
  - assigner **`rc.Result.RequeueAfter = d`** pour satisfaire le requeue minuit.

**Attention** : le package `reconciler` **court-circuite** la chaîne dès qu’un handler pose `Result.RequeueAfter > 0`. Le requeue minuit est posé dans `UpdateStatusHandler` (dernier handler) donc aucun handler n’est court-circuité. Si un futur handler après `UpdateStatus` devait exister, fusionner avec `min(existant, minuit)`.

- Le **`ComponentReconciler.Reconcile`** retourne déjà `rc.Result` si `RequeueAfter > 0` : aucun changement de structure nécessaire.

### Ordre de la chaîne résultant

1. `ValidatePortalRefHandler`
2. `ComputeStatusHandler`
3. **`MergeDailyStatusHandler`** *(nouveau)*
4. `UpdateStatusHandler` *(+ requeue minuit)*

---

## ReadStore / gRPC / UI (front)

- Mettre à jour le ReadStore pour exposer l’historique journalier au front :
  - étendre `internal/domain/component.ComponentView` avec la série journalière ;
  - mapper le champ dans `ToView` (`internal/controller/component/chain/handlers.go`) ;
  - propager la donnée via gRPC/Connect (proto + mapping serveur) ;
  - consommer la nouvelle donnée côté web UI pour l’affichage des 30 jours.

---

## Tests

- **Unitaires domaine** : merge, prune, passage de jour, fenêtre 30 jours, `DurationUntilNextMidnightUTC` (y compris cas limite minuit).
- **Handler / intégration** : si le projet couvre le controller Component — reconcile pose un `RequeueAfter` attendu (mock d’horloge ou valeur bornée).

---

## Cas limites (à documenter dans le plan ou les commentaires API)

| Sujet | Décision proposée |
|--------|-------------------|
| Jour sans aucun reconcile | Ne devrait pas arriver : le requeue minuit garantit un reconcile ~00:05 UTC chaque jour. Si malgré tout un jour est manqué (redémarrage contrôleur, etc.), pas d’entrée pour ce jour (trou acceptable, pas de fill-gaps). |
| Fuseau | **UTC** pour la clé jour et pour la minuit du requeue. |
| Statut inconnu | Rang 0 / ignoré, cohérent avec le domaine actuel. |

---

## Todo

- [ ] Étendre `ComponentStatus` avec `DailyComponentStatus` struct + `dailyWorstStatus` slice (`+listType=map`, `+listMapKey=date`) dans `component_types.go`.
- [ ] Implémenter `DailyStatus` value object + `MergeDailyWorst` + pruning 30 jours + `DurationUntilNextMidnightUTC` en domaine, avec tests unitaires (TDD).
- [ ] Créer `MergeDailyStatusHandler` (nouveau handler dédié) inséré entre `ComputeStatusHandler` et `UpdateStatusHandler`.
- [ ] Ajouter le requeue minuit (`rc.Result.RequeueAfter`) dans `UpdateStatusHandler` (dernier handler), avec horloge injectable pour les tests.
- [ ] Étendre `ComponentView` avec `[]DailyStatus` + mapper dans `ToView` + ReadStore.
- [ ] Étendre proto (`DailyComponentStatus` message) + mappings gRPC/Connect, puis consommer côté front.
- [ ] Exécuter `make helm`, `make doc`, `make test`, `make lint`.

---

## Références code existant

- Chaîne : `internal/controller/component/chain/handlers.go` (`ComputeStatusHandler`, `MergeDailyStatusHandler` *nouveau*, `UpdateStatusHandler`).
- Sévérité : `internal/domain/component/read_model.go` (`StatusSeverityRank`).
- Résultat reconcile : `internal/reconciler/handler.go` (`ReconcileContext.Result`).
- Types CRD : `api/v1alpha1/component_types.go` (`ComponentStatus`, `DailyComponentStatus`).
