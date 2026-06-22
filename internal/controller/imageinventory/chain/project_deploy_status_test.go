/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	tImgBaseNginx     = "docker.io/library/nginx:1.25.0"
	tImgGhcrAPIsemver = "ghcr.io/acme/api:v3.1.0"
	tSourceAcmeAPI    = "https://github.com/acme/api"
	tRevAbc           = "abc123def456"
)

// fakeLabelReader maps "host/repository:reference" -> labels.
type fakeLabelReader struct {
	labels map[string]map[string]string
}

func (f *fakeLabelReader) ImageConfigLabels(_ context.Context, host, repository, reference string) (map[string]string, error) {
	return f.labels[host+"/"+repository+":"+reference], nil
}

func newProjectTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	return scheme
}

// TestProjectDeployStatus_OnlySourceLabeledImages verifies that the handler
// creates a DeployStatus CR whose Spec.Services contains ONLY the
// source-labeled images, with correct SourceRepo and DeployedRef (both the
// revision-label path and the semver-tag fallback path), and skips base images.
func TestProjectDeployStatus_OnlySourceLabeledImages(t *testing.T) {
	t.Parallel()

	scheme := newProjectTestScheme(t)

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: tNsDefault},
	}
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalMain},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(portal, inv).
		Build()

	reader := &fakeLabelReader{labels: map[string]map[string]string{
		// First-party image with explicit revision label.
		"ghcr.io/acme/api:v1.2.3": {
			ociLabelSource:   tSourceAcmeAPI,
			ociLabelRevision: tRevAbc,
		},
		// First-party image with NO revision label but a semver tag → fallback.
		"ghcr.io/acme/api:v3.1.0": {
			ociLabelSource: tSourceAcmeAPI,
		},
		// Base image: no source label → must be skipped.
		"docker.io/library/nginx:1.25.0": {},
	}}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{
		Resource: inv,
		Data: ChainData{Observations: []domainimageregistry.ContainerObservation{
			{WorkloadKind: tKindDeploy, WorkloadName: tNameAPI, WorkloadNamespace: tNsDefault, ContainerName: tNameAPI, PodImage: tImgGhcrAPIv1},
			{WorkloadKind: tKindDeploy, WorkloadName: "api2", WorkloadNamespace: tNsDefault, ContainerName: tNameAPI, PodImage: tImgGhcrAPIsemver},
			{WorkloadKind: tKindDeploy, WorkloadName: tNameWeb, WorkloadNamespace: tNsDefault, ContainerName: tContainerSidecar, PodImage: tImgBaseNginx},
		}},
	}

	h := NewProjectDeployStatusHandler(cli, reader)
	require.NoError(t, h.Handle(context.Background(), rc))

	// One DeployStatus CR for (portal=main, namespace=default).
	name := domainimageregistry.DeployStatusCRName(tPortalMain, tNsDefault)
	got := &sreportalv1alpha1.DeployStatus{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: name, Namespace: tNsDefault}, got))

	require.Equal(t, tPortalMain, got.Spec.PortalRef)
	require.Equal(t, tNsDefault, got.Spec.Namespace)
	require.Len(t, got.Spec.Services, 2, "only the two source-labeled images, base image skipped")

	byImage := map[string]sreportalv1alpha1.DeployStatusEntry{}
	for _, s := range got.Spec.Services {
		byImage[s.Image] = s
		require.Equal(t, tSourceAcmeAPI, s.SourceRepo)
		require.NotEmpty(t, s.Key)
		require.Empty(t, s.State, "State must not be set on Spec (controller-managed input)")
	}

	// Revision-label path.
	require.Equal(t, tRevAbc, byImage[tImgGhcrAPIv1].DeployedRef)
	// Semver-tag fallback path.
	require.Equal(t, "v3.1.0", byImage[tImgGhcrAPIsemver].DeployedRef)

	// Owner reference set for GC.
	require.Len(t, got.OwnerReferences, 1)
	require.Equal(t, tNameInv, got.OwnerReferences[0].Name)
}

// TestProjectDeployStatus_FeatureDisabledSkips verifies that disabling the
// deployStatus feature on the portal skips projection entirely.
func TestProjectDeployStatus_FeatureDisabledSkips(t *testing.T) {
	t.Parallel()

	scheme := newProjectTestScheme(t)

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Features: &sreportalv1alpha1.PortalFeatures{DeployStatus: new(bool)},
		},
	}
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalMain},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, inv).Build()

	reader := &fakeLabelReader{labels: map[string]map[string]string{
		"ghcr.io/acme/api:v1.2.3": {ociLabelSource: tSourceAcmeAPI, ociLabelRevision: tRevAbc},
	}}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{
		Resource: inv,
		Data: ChainData{Observations: []domainimageregistry.ContainerObservation{
			{WorkloadKind: tKindDeploy, WorkloadName: tNameAPI, WorkloadNamespace: tNsDefault, ContainerName: tNameAPI, PodImage: tImgGhcrAPIv1},
		}},
	}

	h := NewProjectDeployStatusHandler(cli, reader)
	require.NoError(t, h.Handle(context.Background(), rc))

	// No DeployStatus CR must exist.
	var list sreportalv1alpha1.DeployStatusList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items, "feature disabled → no projection")
}
