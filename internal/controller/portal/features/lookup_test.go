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

package features

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestLookupPortalFeature(t *testing.T) {
	t.Parallel()

	sch := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(sch))

	trueRef := true
	enabledPortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Features: &sreportalv1alpha1.PortalFeatures{DNS: &trueRef},
		},
	}
	falseRef := false
	disabledPortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled", Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Features: &sreportalv1alpha1.PortalFeatures{DNS: &falseRef},
		},
	}

	isDNS := func(f *sreportalv1alpha1.PortalFeatures) bool { return f.IsDNSEnabled() }

	tests := []struct {
		name        string
		portalName  string
		clientErr   error
		wantEnabled bool
		wantErr     bool
	}{
		{name: "missing portal returns disabled without error", portalName: "ghost", wantEnabled: false, wantErr: false},
		{name: "enabled feature returns true", portalName: tPortalMain, wantEnabled: true, wantErr: false},
		{name: "disabled feature returns false without error", portalName: "disabled", wantEnabled: false, wantErr: false},
		{name: "transient error surfaces", portalName: tPortalMain, clientErr: errors.New("etcdserver: timeout"), wantEnabled: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builder := fake.NewClientBuilder().
				WithScheme(sch).
				WithObjects(enabledPortal, disabledPortal)

			if tc.clientErr != nil {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return tc.clientErr
					},
				})
			}

			c := builder.Build()
			enabled, err := LookupPortalFeature(context.Background(), c, tNsDefault, tc.portalName, isDNS)
			if tc.wantErr {
				require.Error(t, err)
				require.False(t, apierrors.IsNotFound(err))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantEnabled, enabled)
		})
	}

	// Sanity: schema registered the GVK we expect (compile guard, keeps `schema` import live).
	_ = schema.GroupVersionKind{Group: sreportalv1alpha1.GroupVersion.Group, Version: sreportalv1alpha1.GroupVersion.Version, Kind: "Portal"}
}
