package image

import (
	"context"
	"testing"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestStoreReplaceAndDeduplicate(t *testing.T) {
	t.Parallel()

	s := NewStore()
	err := s.Replace(context.Background(), "portal-a", []domainimage.ImageView{
		{
			PortalRef:  "portal-a",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.2.3",
			TagType:    domainimage.TagTypeSemver,
			Workloads: []domainimage.WorkloadRef{
				{Kind: "Deployment", Namespace: "default", Name: "a", Container: "web"},
			},
		},
		{
			PortalRef:  "portal-a",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.2.3",
			TagType:    domainimage.TagTypeSemver,
			Workloads: []domainimage.WorkloadRef{
				{Kind: "Deployment", Namespace: "default", Name: "b", Container: "web"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	out, err := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
	if len(out[0].Workloads) != 2 {
		t.Fatalf("workloads=%d want 2", len(out[0].Workloads))
	}
}

func TestStoreFilters(t *testing.T) {
	t.Parallel()
	s := NewStore()
	_ = s.Replace(context.Background(), "portal-a", []domainimage.ImageView{
		{PortalRef: "portal-a", Registry: "docker.io", Repository: "library/nginx", Tag: "latest", TagType: domainimage.TagTypeLatest},
		{PortalRef: "portal-a", Registry: "ghcr.io", Repository: "org/app", Tag: "1.0.0", TagType: domainimage.TagTypeSemver},
	})
	out, err := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a", Registry: "ghcr.io", TagType: "semver", Search: "org/"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(out) != 1 || out[0].Repository != "org/app" {
		t.Fatalf("unexpected output: %+v", out)
	}
}
