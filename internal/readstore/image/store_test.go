package image

import (
	"context"
	"sync"
	"testing"
	"time"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func mkView(portal, repo, tag string, wks ...domainimage.WorkloadRef) domainimage.ImageView {
	return domainimage.ImageView{
		PortalRef:     portal,
		Registry:      tHostDocker,
		Repository:    repo,
		Tag:           tag,
		TagType:       domainimage.TagTypeSemver,
		OriginalImage: tHostDocker + "/" + repo + ":" + tag,
		MutatedImage:  tHostDocker + "/" + repo + ":" + tag,
		ChangeType:    tChangeTypeNone,
		Workloads:     wks,
	}
}

func wk(ns, name string) domainimage.WorkloadRef {
	return domainimage.WorkloadRef{Kind: tKindDeploy, Namespace: ns, Name: name, Container: "app"}
}

func TestReplaceForNamespace_Single(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	views := []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	}
	if err := s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, views); err != nil {
		t.Fatalf("ReplaceForNamespace: %v", err)
	}

	out, err := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1", len(out))
	}
	if out[0].Repository != tRepoNginx || out[0].Tag != tVersion123 {
		t.Fatalf("unexpected entry: %+v", out[0])
	}
	if len(out[0].Workloads) != 1 {
		t.Fatalf("workloads=%d, want 1", len(out[0].Workloads))
	}
}

func TestReplaceForNamespace_AggregatesAcrossNamespaces(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	// Same image (portal/registry/repo/tag) used in two namespaces — must aggregate
	// into a single entry with both workloads.
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	})
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsOther, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsOther, "web2")),
	})

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1 (deduped)", len(out))
	}
	if len(out[0].Workloads) != 2 {
		t.Fatalf("workloads=%d, want 2", len(out[0].Workloads))
	}
}

func TestReplaceForNamespace_OverwriteSameScope(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion100, wk(tNsDefault, tWorkloadWeb)),
	})
	// Replace contribution in same scope: nginx 1.0.0 must be removed, redis 7.0 added.
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, "library/redis", "7.0.0", wk(tNsDefault, "cache")),
	})

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 1 || out[0].Repository != "library/redis" {
		t.Fatalf("want [library/redis], got %+v", out)
	}
}

func TestReplaceForNamespace_PreservesOtherNamespaceContribution(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	})
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsOther, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsOther, "web2")),
	})

	// Replace `default`-scope contribution — `other`-scope contribution must stay.
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, "library/redis", "7.0.0", wk(tNsDefault, "cache")),
	})

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 2 {
		t.Fatalf("len=%d, want 2 (nginx + redis)", len(out))
	}
}

func TestReplaceForNamespace_LatestVersionMetadataPropagates(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()
	now := time.Now().UTC()

	view := domainimage.ImageView{
		PortalRef:        tPortalA,
		Registry:         tHostDocker,
		Repository:       tRepoNginx,
		Tag:              tVersion123,
		TagType:          domainimage.TagTypeSemver,
		OriginalImage:    tImgNginxDocker123,
		MutatedImage:     tImgNginxDocker123,
		ChangeType:       tChangeTypeNone,
		LatestVersion:    tVersion124,
		LatestCheckedAt:  &now,
		UpgradeAvailable: true,
		Workloads:        []domainimage.WorkloadRef{wk(tNsDefault, tWorkloadWeb)},
	}
	if err := s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{view}); err != nil {
		t.Fatalf("ReplaceForNamespace: %v", err)
	}

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1", len(out))
	}
	got := out[0]
	if got.LatestVersion != tVersion124 || !got.UpgradeAvailable {
		t.Fatalf("LatestVersion=%q UpgradeAvailable=%v, want 1.2.4 true", got.LatestVersion, got.UpgradeAvailable)
	}
	if got.LatestCheckedAt == nil || !got.LatestCheckedAt.Equal(now) {
		t.Fatalf("LatestCheckedAt=%v, want %v", got.LatestCheckedAt, now)
	}
	if got.OriginalImage == "" || got.MutatedImage == "" || got.ChangeType == "" {
		t.Fatalf("missing original/mutated/changeType: %+v", got)
	}
}

func TestReplaceForNamespace_EmptyClearsScope(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	})
	if err := s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, nil); err != nil {
		t.Fatalf("ReplaceForNamespace empty: %v", err)
	}
	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 0 {
		t.Fatalf("want empty after replace with nil, got %+v", out)
	}
}

func TestRemoveForNamespace(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	})
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsOther, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsOther, "web2")),
	})

	if err := s.RemoveForNamespace(ctx, tPortalA, tHostDocker, tNsDefault); err != nil {
		t.Fatalf("RemoveForNamespace: %v", err)
	}

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1 (only `other` namespace contribution remains)", len(out))
	}
	if len(out[0].Workloads) != 1 || out[0].Workloads[0].Namespace != tNsOther {
		t.Fatalf("unexpected workload: %+v", out[0].Workloads)
	}
}

func TestRemoveForNamespace_Idempotent(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	if err := s.RemoveForNamespace(ctx, tPortalA, tHostDocker, tNsDefault); err != nil {
		t.Fatalf("RemoveForNamespace on empty: %v", err)
	}
	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 0 {
		t.Fatalf("want empty, got %+v", out)
	}
}

func TestPortalIsolation(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
	})
	_ = s.ReplaceForNamespace(ctx, tPortalB, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalB, "library/redis", "7.0.0", wk(tNsDefault, "cache")),
	})

	a, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	b, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalB})
	if len(a) != 1 || a[0].Repository != tRepoNginx {
		t.Fatalf("portal A: want [library/nginx], got %+v", a)
	}
	if len(b) != 1 || b[0].Repository != "library/redis" {
		t.Fatalf("portal B: want [library/redis], got %+v", b)
	}
}

func TestListFilters(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	views := []domainimage.ImageView{
		{
			PortalRef:  tPortalA,
			Registry:   tHostDocker,
			Repository: tRepoNginx,
			Tag:        "latest",
			TagType:    domainimage.TagTypeLatest,
		},
		{
			PortalRef:  tPortalA,
			Registry:   tHostGhcr,
			Repository: "org/app",
			Tag:        tVersion100,
			TagType:    domainimage.TagTypeSemver,
		},
	}
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, views[:1])
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostGhcr, tNsDefault, views[1:])

	out, err := s.List(ctx, domainimage.ImageFilters{
		Portal: tPortalA, Registry: tHostGhcr, TagType: "semver", Search: "org/",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].Repository != "org/app" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestList_SortedDeterministically(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostGhcr, tNsDefault, []domainimage.ImageView{
		{PortalRef: tPortalA, Registry: tHostGhcr, Repository: "org/b", Tag: tVersion100},
	})
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		{PortalRef: tPortalA, Registry: tHostDocker, Repository: tRepoLibA, Tag: tVersion100},
		{PortalRef: tPortalA, Registry: tHostDocker, Repository: tRepoLibA, Tag: "0.9.0"},
	})

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 3 {
		t.Fatalf("len=%d, want 3", len(out))
	}
	// Expected sort: registry asc, repo asc, tag asc.
	want := []struct{ reg, repo, tag string }{
		{tHostDocker, tRepoLibA, "0.9.0"},
		{tHostDocker, tRepoLibA, tVersion100},
		{tHostGhcr, "org/b", tVersion100},
	}
	for i, w := range want {
		if out[i].Registry != w.reg || out[i].Repository != w.repo || out[i].Tag != w.tag {
			t.Fatalf("entry %d: got %+v want %+v", i, out[i], w)
		}
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
		mkView(tPortalA, "library/redis", "7.0.0", wk(tNsDefault, "cache")),
	})
	n, err := s.Count(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Fatalf("count=%d, want 2", n)
	}
}

func TestSubscribeBroadcastsOnMutation(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()
	ch := s.Subscribe()

	go func() {
		_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
			mkView(tPortalA, tRepoNginx, tVersion123, wk(tNsDefault, tWorkloadWeb)),
		})
	}()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("did not receive broadcast")
	}
}

func TestConcurrentReadDuringReplace(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
		mkView(tPortalA, tRepoNginx, tVersion100, wk(tNsDefault, tWorkloadWeb)),
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			_, _ = s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{
				mkView(tPortalA, tRepoNginx, tVersion100, wk(tNsDefault, tWorkloadWeb)),
			})
		}
	}()
	wg.Wait()
}

func TestReplaceForNamespace_AggregatesLatestVersionAcrossContributions(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ctx := context.Background()

	now := time.Now().UTC()
	earlier := now.Add(-time.Hour)

	// First contribution has older LatestCheckedAt + LatestVersion.
	v1 := domainimage.ImageView{
		PortalRef: tPortalA, Registry: tHostDocker, Repository: tRepoNginx, Tag: tVersion123,
		TagType:         domainimage.TagTypeSemver,
		OriginalImage:   tImgNginxDocker123,
		MutatedImage:    tImgNginxDocker123,
		ChangeType:      tChangeTypeNone,
		LatestVersion:   tVersion124,
		LatestCheckedAt: &earlier,
		Workloads:       []domainimage.WorkloadRef{wk(tNsDefault, tWorkloadWeb)},
	}
	v2 := domainimage.ImageView{
		PortalRef: tPortalA, Registry: tHostDocker, Repository: tRepoNginx, Tag: tVersion123,
		TagType:         domainimage.TagTypeSemver,
		OriginalImage:   tImgNginxDocker123,
		MutatedImage:    tImgNginxDocker123,
		ChangeType:      tChangeTypeNone,
		LatestVersion:   "1.2.5",
		LatestCheckedAt: &now,
		Workloads:       []domainimage.WorkloadRef{wk(tNsOther, "web2")},
	}
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsDefault, []domainimage.ImageView{v1})
	_ = s.ReplaceForNamespace(ctx, tPortalA, tHostDocker, tNsOther, []domainimage.ImageView{v2})

	out, _ := s.List(ctx, domainimage.ImageFilters{Portal: tPortalA})
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1", len(out))
	}
	// Newest LatestCheckedAt wins.
	if out[0].LatestVersion != "1.2.5" {
		t.Fatalf("LatestVersion=%q, want 1.2.5", out[0].LatestVersion)
	}
	if out[0].LatestCheckedAt == nil || !out[0].LatestCheckedAt.Equal(now) {
		t.Fatalf("LatestCheckedAt=%v, want %v", out[0].LatestCheckedAt, now)
	}
}
