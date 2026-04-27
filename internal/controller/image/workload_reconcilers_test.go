package image

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestDeploymentReconcilerUpsertsOnPresent(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, dep).Build()
	writer := &fakeImageWriter{}
	r := &DeploymentImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want 1 replace, got %d", len(writer.replaces))
	}
}

func TestDeploymentReconcilerDeletesOnNotFound(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{}
	r := &DeploymentImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "gone"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.deletes) != 1 {
		t.Fatalf("want 1 delete, got %d", len(writer.deletes))
	}
	want := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "gone"}
	if writer.deletes[0] != want {
		t.Fatalf("delete=%+v want %+v", writer.deletes[0], want)
	}
}

func TestCronJobReconcilerExtractsJobTemplatePodSpec(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "run", Image: "ghcr.io/acme/cron:v1"}}},
					},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, cj).Build()
	writer := &fakeImageWriter{}
	r := &CronJobImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "nightly"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want 1 replace, got %d", len(writer.replaces))
	}
	if writer.replaces[0].wk.Kind != "CronJob" {
		t.Fatalf("kind=%q want CronJob", writer.replaces[0].wk.Kind)
	}
}
