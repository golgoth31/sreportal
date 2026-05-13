# Image Tag-Format Matching

## Problem

`ResolveLatestVersionsHandler` picks the highest semver across **all** tags of a
repository. This produces incorrect "upgrade available" signals when a repository
exposes multiple **variants** of the same release:

- Cluster runs `nginx:1.25.0-alpine` → controller suggests `nginx:1.27.0`
  (different image variant: debian-based vs alpine-based).
- Cluster runs `redis:v7.2.0` (with `v` prefix) → controller may pick `7.2.4`
  (lost prefix).
- Cluster runs `postgres:18.3` (2-segment) → `ClassifyTag` returns `other`,
  resolution is skipped entirely.

Additionally, the UI shows nothing in the version row when `tagType != semver`,
which is ambiguous: same blank cell whether the lookup never ran, failed, or is
intentionally not tracked.

## Goal

1. When resolving the latest version, **filter candidate tags to the same
   variant** as the current tag (same prefix, same suffix flavor).
2. Accept **2-segment semver tags** (`18.3`) as `TagTypeSemver`.
3. **Compare across segment counts** so `18.3` can upgrade to `18.4` or `18.3.0`
   (loose on segments, strict on variant).
4. **Surface `TagType` in the UI** with an explicit hint for non-semver tags
   ("rolling tag — not tracked").

## Non-goals

- Digest drift detection for floating tags (`nginx:alpine`, `mainline`,
  `latest`). Out of scope — separate feature, requires new spec field
  (`OriginalDigest`), manifest HEAD calls, different storage model.
- New tag classifications. The 5 existing `TagType` values stay.
- Proto / gRPC / CRD changes. `TagType` is already propagated end-to-end.

## Design

### 1. New domain type: `TagPattern`

File: `internal/domain/imageregistry/tagpattern.go`

```go
type TagPattern struct {
    Prefix   string // "v" or ""
    Suffix   string // "-alpine", "-bookworm", "-jammy", ...  (variant flavor)
    HasPre   bool   // tag carries a SemVer pre-release segment (-rc.N, -beta.N)
}

func ExtractPattern(tag string) (TagPattern, bool)
func (p TagPattern) Matches(candidate string) bool
```

**Decomposition regex**:
```
^(v)?(\d+(?:\.\d+){0,2})((?:[-+][0-9A-Za-z.+-]+)?)$
```
- group 1: `prefix` (`v` or empty)
- group 2: numeric core (`18`, `18.3`, `18.3.0`)
- group 3: trailing modifier (may be empty)

The group-3 modifier is then split into:
- **pre-release SemVer**: matches `-(alpha|beta|rc|pre|dev|snapshot|m\d+)([.\-]\w+)*` → `HasPre=true`, treated as part of version (compared as SemVer pre-release).
- **variant flavor**: anything else → stored in `Suffix`.

`Matches(candidate)` returns true iff `ExtractPattern(candidate)` yields the
same `Prefix`, the same `Suffix`, and (if `p.HasPre`) candidate must also be a
pre-release. Segment count is **not** compared — `18.3` matches `18.4` and
`18.3.0`.

### 2. Widen `ClassifyTag`

File: `internal/domain/image/parser.go`

```go
semverRE = regexp.MustCompile(`^v?\d+(?:\.\d+){1,2}(?:[-+][0-9A-Za-z\.-]+)?$`)
```

Now matches `18`, `18.3`, `18.3.0`, `v1.2`, `1.2-rc.1`. Rejects `1`, `1.2.3.4`.

Risk: low. The regex is only applied in `ClassifyTag`, which is only called on
already-parsed OCI tag strings (`go-containerregistry`).

### 3. Pad `canonicalSemver`

File: `internal/domain/imageregistry/version.go`

`golang.org/x/mod/semver` requires `major.minor.patch`. Pad missing segments
with `.0` for the comparison, but keep the original tag for return values.

```go
func canonicalSemver(tag string) string {
    if tag == "" { return "" }
    c := strings.TrimPrefix(tag, "v")
    main, mod := splitMainAndModifier(c) // "" or "-rc.1"/"+build"
    parts := strings.Split(main, ".")
    for len(parts) < 3 { parts = append(parts, "0") }
    rebuilt := "v" + strings.Join(parts, ".") + mod
    if !semver.IsValid(rebuilt) { return "" }
    return rebuilt
}
```

### 4. New entry point: `PickLatestMatching`

File: `internal/domain/imageregistry/version.go`

```go
// PickLatestMatching scans `tags` keeping only candidates that share the same
// TagPattern as `originalTag`, then returns the highest SemVer among them.
//
// When `originalTag` cannot be parsed as a TagPattern, the function returns
// ("", 0, false) — no fallback to cross-variant comparison.
func PickLatestMatching(tags []string, originalTag string) (latest string, rejected int, found bool)
```

`PickLatestSemver` is **kept** (used in tests, public API). `PickLatestMatching`
is the new caller-facing entry point.

### 5. Wire call site

File: `internal/controller/imageregistry/chain/resolve_latest_versions.go:264`

```go
latest, rejected, found := domainimageregistry.PickLatestMatching(tags, img.Spec.OriginalTag)
```

No other change in the chain. `IsUpgrade` keeps working as-is (variant suffix
filtered out before comparison).

### 6. UI hint for non-semver

File: `web/src/features/image/ui/ImageCard.tsx` (around line 162)

Replace the bare `tagType === "semver"` block by:

```tsx
{image.tagType === "semver" ? (
  <span className="font-mono text-[11px] text-muted-foreground">
    {image.latestVersion ? (...) : <span className="opacity-40">latest: —</span>}
  </span>
) : (
  <span className="text-[11px] text-muted-foreground opacity-60">
    {tagTypeHint(image.tagType)}
  </span>
)}
```

```ts
// image.badge-utils.ts
export function tagTypeHint(t: TagType): string {
  switch (t) {
    case "latest": return "rolling tag — not tracked";
    case "other":  return "rolling tag — not tracked";
    case "digest": return "digest-pinned";
    case "commit": return "commit-pinned";
    default:       return "";
  }
}
```

## Test plan

### Go

`internal/domain/imageregistry/tagpattern_test.go` (new):
- `1.25.3-alpine` → `{Prefix:"", Suffix:"-alpine", HasPre:false}`
- `v1.2` → `{Prefix:"v", Suffix:"", HasPre:false}`
- `1.2.3-rc.1` → `{Prefix:"", Suffix:"", HasPre:true}`
- `1.2.3-bookworm-slim` → `{Prefix:"", Suffix:"-bookworm-slim", HasPre:false}`
- `18.3` → `{Prefix:"", Suffix:"", HasPre:false}`
- `alpine` → `(_, false)`
- `Matches`: `1.25.0-alpine` matches `1.25.3-alpine`, not `1.25.3` or `1.25.3-debian`.
- `Matches`: `18.3` matches `18.4` and `18.3.0` (loose on segments).
- `Matches` on pre-release track: `1.2.3-rc.1` matches `1.2.4-rc.2` but not `1.2.4`.

`internal/domain/imageregistry/version_test.go` (extend):
- `PickLatestMatching([1.25.3, 1.25.3-alpine, 1.27.0], "1.25.0-alpine")` → `1.25.3-alpine`.
- `PickLatestMatching([15.1, 15.1-alpine, 16-alpine], "15-alpine")` → `16-alpine`.
- `PickLatestMatching([7.2.4, v7.2.4], "v7.2.0")` → `v7.2.4`.
- `PickLatestMatching([18.3, 18.4, 18.3.0, 18.3-alpine, 17.5], "18.3")` → `18.4`.
- `PickLatestMatching([1.2.3, 1.2.4-rc.2], "1.2.3-rc.1")` → `1.2.4-rc.2`.
- `PickLatestMatching([1.2.3], "alpine")` → `("", 0, false)`.

`internal/domain/image/parser_test.go` (extend):
- `ClassifyTag("18.3")` → `TagTypeSemver` (regression: was `TagTypeOther`).
- `ClassifyTag("18")` → `TagTypeSemver`.
- `ClassifyTag("alpine")` → `TagTypeOther`.

### TypeScript

`web/src/features/image/ui/image.badge-utils.test.ts` (extend):
- `tagTypeHint("other")` returns "rolling tag — not tracked".
- `tagTypeHint("semver")` returns "".

## Files touched

| Path | Action |
|---|---|
| `internal/domain/imageregistry/tagpattern.go` | new |
| `internal/domain/imageregistry/tagpattern_test.go` | new |
| `internal/domain/imageregistry/version.go` | edit (canonicalSemver pad + PickLatestMatching) |
| `internal/domain/imageregistry/version_test.go` | extend |
| `internal/domain/image/parser.go` | edit (semverRE) |
| `internal/domain/image/parser_test.go` | extend |
| `internal/controller/imageregistry/chain/resolve_latest_versions.go` | 1-line edit (call site) |
| `internal/controller/imageregistry/chain/resolve_latest_versions_test.go` | adjust if it pins call signature |
| `web/src/features/image/ui/image.badge-utils.ts` | add `tagTypeHint` |
| `web/src/features/image/ui/ImageCard.tsx` | render hint for non-semver |
| `web/src/features/image/ui/image.badge-utils.test.ts` | extend |

No proto, no CRD, no manifests, no RBAC changes.

## Migration / rollout

Pure code change. Next reconciliation re-runs resolution and overwrites
`Status.Images[].LatestVersion` with the variant-correct value. Stale
`UpgradeAvailable=true` entries from before the fix self-heal on next pass.

## Risks

- **`semverRE` widening**: `1.2-alpine` would now classify as `TagTypeOther`
  (regex tail rejects `-alpine` after 2-segment). Wait — needs verification: the
  current regex is anchored, `-alpine` after `1.2` is allowed by the
  `(?:[-+]...)?` group. So `1.2-alpine` classifies as `TagTypeSemver`. The
  variant filter then strips it correctly. **Verify with test.**
- **Pre-release detection heuristic**: a tag like `1.2.3-mycorp` is ambiguous —
  is `mycorp` a pre-release identifier or a variant flavor? Heuristic only
  treats well-known prefixes (`alpha`, `beta`, `rc`, `pre`, `dev`, `snapshot`,
  `m<n>`) as pre-release. Custom pre-release labels fall into "variant" bucket
  — acceptable trade-off (the operator would tag those builds as variants
  anyway).
- **Loose-on-segments rule**: when a repo migrates from 2-segment to 3-segment
  (`18.3` → `18.4.0`), the upgrade is reported. But what if the registry keeps
  both `18.4` and `18.4.0` (alias)? `PickLatestMatching` picks the higher
  canonical SemVer — they compare equal after padding (`v18.4.0` == `v18.4.0`),
  so one of them is returned (whichever appears first in tag iteration). This
  is acceptable but non-deterministic. Could be tightened later if it bites.

## Out of scope (for future work)

- Digest drift detection for `latest`/`alpine`/`mainline` (separate spec field,
  `HEAD /v2/<repo>/manifests/<tag>`, store + compare digest).
- Per-image `Spec.Images[].TagFormat: strict|loose` override.
- Surfacing `rejected` count in the CR status (currently only emitted as a
  metric).
