package image

import (
	"context"
	"sync"
	"testing"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func mkView(portal, repo, tag string, wk domainimage.WorkloadKey, container string) domainimage.ImageView {
	return domainimage.ImageView{
		PortalRef:  portal,
		Registry:   "docker.io",
		Repository: repo,
		Tag:        tag,
		TagType:    domainimage.TagTypeSemver,
		Workloads: []domainimage.WorkloadRef{{
			Kind: wk.Kind, Namespace: wk.Namespace, Name: wk.Name, Container: container,
		}},
	}
}

func TestReplaceWorkloadAggregatesAcrossWorkloads(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wkA := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	wkB := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "b"}

	if err := s.ReplaceWorkload(context.Background(), "portal-a", wkA, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wkA, "web"),
	}); err != nil {
		t.Fatalf("ReplaceWorkload A: %v", err)
	}
	if err := s.ReplaceWorkload(context.Background(), "portal-a", wkB, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wkB, "web"),
	}); err != nil {
		t.Fatalf("ReplaceWorkload B: %v", err)
	}

	out, err := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1 (deduplicated)", len(out))
	}
	if len(out[0].Workloads) != 2 {
		t.Fatalf("workloads=%d want 2", len(out[0].Workloads))
	}
}

func TestReplaceWorkloadOverwritesSameKey(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.4", wk, "web"),
	})

	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 1 || out[0].Tag != "1.2.4" {
		t.Fatalf("want [1.2.4], got %+v", out)
	}
}

func TestDeleteWorkloadAllPortals(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	_ = s.ReplaceWorkload(context.Background(), "portal-b", wk, []domainimage.ImageView{
		mkView("portal-b", "library/nginx", "1.2.3", wk, "web"),
	})

	if err := s.DeleteWorkloadAllPortals(context.Background(), wk); err != nil {
		t.Fatalf("DeleteWorkloadAllPortals: %v", err)
	}

	for _, portal := range []string{"portal-a", "portal-b"} {
		out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: portal})
		if len(out) != 0 {
			t.Fatalf("portal %s still has entries: %+v", portal, out)
		}
	}
}

func TestReplaceAllSwapsPortalAtomically(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk1 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	wk2 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "b"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk1, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.0.0", wk1, "web"),
	})

	byWorkload := map[domainimage.WorkloadKey][]domainimage.ImageView{
		wk2: {mkView("portal-a", "library/redis", "7.0.0", wk2, "cache")},
	}
	if err := s.ReplaceAll(context.Background(), "portal-a", byWorkload); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 1 || out[0].Repository != "library/redis" {
		t.Fatalf("want [library/redis], got %+v", out)
	}
}

func TestDeletePortal(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	if err := s.DeletePortal(context.Background(), "portal-a"); err != nil {
		t.Fatalf("DeletePortal: %v", err)
	}
	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 0 {
		t.Fatalf("portal-a still has entries: %+v", out)
	}
}

func TestListFilters(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		{PortalRef: "portal-a", Registry: "docker.io", Repository: "library/nginx", Tag: "latest", TagType: domainimage.TagTypeLatest},
		{PortalRef: "portal-a", Registry: "ghcr.io", Repository: "org/app", Tag: "1.0.0", TagType: domainimage.TagTypeSemver},
	})

	out, err := s.List(context.Background(), domainimage.ImageFilters{
		Portal: "portal-a", Registry: "ghcr.io", TagType: "semver", Search: "org/",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].Repository != "org/app" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestConcurrentReadDuringReplaceAll(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.0.0", wk, "web"),
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			_, _ = s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			_ = s.ReplaceAll(context.Background(), "portal-a", map[domainimage.WorkloadKey][]domainimage.ImageView{
				wk: {mkView("portal-a", "library/nginx", "1.0.0", wk, "web")},
			})
		}
	}()
	wg.Wait()
}
