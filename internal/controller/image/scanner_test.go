package image

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestImageViewsFromPodSpec(t *testing.T) {
	t.Parallel()

	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "main", Image: "ghcr.io/acme/api:v1.2.3"},
		},
		InitContainers: []corev1.Container{
			{Name: "init", Image: "ghcr.io/acme/migrate:latest"},
		},
	}
	got := imageViewsFromPodSpec("main", "Deployment", "default", "api", spec)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].TagType != domainimage.TagTypeSemver {
		t.Fatalf("tagType0=%q", got[0].TagType)
	}
	if got[1].TagType != domainimage.TagTypeLatest {
		t.Fatalf("tagType1=%q", got[1].TagType)
	}
}
