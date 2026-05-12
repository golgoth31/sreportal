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

package chain_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/portal/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"

	"github.com/stretchr/testify/require"
)

func TestFetchRemoteDataHandlerGuardsNilRemoteClient(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := chain.NewFetchRemoteDataHandler(cli)

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-portal", Namespace: nsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: "https://example", Portal: tPortalMain},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{
		Resource: portal,
		Data:     chain.ChainData{RemoteClient: nil},
	}

	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, ctrl.Result{}, rc.Result, "no RequeueAfter should be set when RemoteClient is nil")
	require.Nil(t, rc.Data.FetchResult, "FetchResult should remain nil when RemoteClient is nil")
}
