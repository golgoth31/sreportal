# Plan — Restaurer l'origine (OriginRef) sur les cartes FQDN auto

## Contexte / régression

Depuis la migration v1alpha2 (PR #274), `DNSRecord.spec.entries` est le canal
canonique. La carte FQDN (`web/src/features/dns/ui/FqdnCard.tsx:129`) affiche la
ligne `kind/namespace/name` uniquement si `fqdn.originRef` est présent.

`OriginRef` est dérivé du label external-dns `resource` porté par l'endpoint
(`adapter/endpoint.go:625`, via `EndpointStatusToGroupsV2`). Or :

- `DNSRecordEntry` n'a aucun champ d'origine.
- `endpointsToEntries` (`upsert_dnsrecords.go`) ne propage que FQDN/RecordType/
  Targets/Group → le label `resource` est perdu à l'entrée dans `spec.entries`.
- `MaterialiseEntriesHandler` reconstruit `status.Endpoints` depuis les entries
  et ne réinjecte que `sreportal.io/group`.

⇒ Pour les records `auto` (external-dns), `OriginRef = nil` → la ligne d'origine
disparaît. Le badge `External DNS`/`Manual` (issu de `spec.Origin`) n'est pas
affecté.

## Garantie de cohérence avec la priorité des sources

`IntraDNSDedupHandler` (`intra_dns_dedup.go`) applique `spec.sources.priority`
**avant** la construction des entries et garde l'endpoint **entier** du kind
gagnant pour chaque `(name, recordType)` (dédup globale). Chaque FQDN ne survit
que dans le DNSRecord du kind gagnant. Donc l'origine portée par l'entry =
ressource de la source gagnante, et targets + origine viennent toujours du même
endpoint (pas de mismatch possible). Caveat pré-existant inchangé : départage
intra-kind = ordre du builder de source.

## Objectif (critère de succès)

Une carte d'un FQDN découvert par external-dns réaffiche
`service/<ns>/<name>`, et pour un FQDN produit par 2 kinds, l'origine est celle
du kind prioritaire.

## Changements

### 1. Schéma — `api/v1alpha2/dnsrecord_types.go`
Ajouter à `DNSRecordEntry` un champ optionnel :
```go
// originRef identifies the source Kubernetes resource (format
// "kind/namespace/name") for auto-discovered entries. Empty for manual.
// +optional
OriginRef string `json:"originRef,omitempty"`
```
- Pas de changement de conversion (v1alpha1 embarque le type et round-trip via
  annotation — le champ suit automatiquement).
- **Verify** : `make manifests && make helm && make doc` régénèrent CRD/docs ;
  `go build ./...`.

### 2. Population à l'écriture — `internal/controller/dns/chain/upsert_dnsrecords.go`
Dans `endpointsToEntries`, à la création de l'entry (premier endpoint de la clé,
unicité garantie par le dedup) :
```go
if r := e.Labels[endpoint.ResourceLabelKey]; r != "" {
    entry.OriginRef = r
}
```
- **Verify** : test unitaire `endpointsToEntries` (origine = label `resource`).

### 3. Réinjection à la matérialisation — `internal/controller/dnsrecords/chain/materialise_entries.go`
Lors du build de `EndpointStatus`, si `e.OriginRef != ""`, ajouter au map labels
`labels[endpoint.ResourceLabelKey] = e.OriginRef` (à côté du group label).
- Le hash exclut déjà `ResourceLabelKey` (`adapter/hash.go:34`) → pas de churn de
  reconcile.
- **Verify** : test `MaterialiseEntriesHandler` (status.Labels porte `resource`,
  `EndpointsHash` stable).

### 4. Aucune modif
adapter (`EndpointStatusToGroupsV2`), gRPC (`dns_service.go:217`), proto
(`origin_ref`), web (`FqdnCard.tsx`) : déjà câblés.

## Tests

- `endpointsToEntries` : label `resource` → `entry.OriginRef`.
- `MaterialiseEntriesHandler` : `entry.OriginRef` → `status.Labels[resource]` ;
  `EndpointsHash` inchangé.
- `DNSRecordToFQDNViews` (`project_store_test`) : entry avec `OriginRef` →
  `FQDNView.OriginRef` peuplé.
- Priorité : 2 kinds produisent le même FQDN → l'entry du kind prioritaire porte
  l'origine de ce kind (assoit la garantie ci-dessus).
- Conversion round-trip v1alpha1↔v2 : reste verte (champ présent, préservé).

## Comportement runtime / migration

- 1er reconcile post-déploiement : les `spec.entries` des DNSRecord auto gagnent
  `originRef` (un diff `CreateOrUpdate` unique). Records `manual` : `OriginRef`
  vide → pas de ligne d'origine (comportement inchangé).
- Changement CRD purement additif/optionnel (pas de breaking).

## Vérification finale

- `make manifests helm doc` propre ; `go build ./...` ;
  `go test ./api/... ./internal/controller/dns/... ./internal/controller/dnsrecords/...`
  ; `golangci-lint run`.
- Live (recommandé) : déployer, vérifier qu'une carte external-dns réaffiche la
  ligne d'origine, et qu'un FQDN multi-sources montre l'origine du kind
  prioritaire.

## Hors scope

- Permettre à l'utilisateur de renseigner `originRef` sur un record `manual`
  (follow-up éventuel).
- Toute modif proto/web (aucune nécessaire).
