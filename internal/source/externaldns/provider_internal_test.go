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

package externaldns

import (
	"context"
	"errors"
	"testing"
	"time"

	kubefake "k8s.io/client-go/kubernetes/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// TestEndpoints_BoundedWaitReturnsNotReady verifies that a build which has not
// finished within the bounded wait surfaces ErrSourceNotReady instead of
// blocking — the core of the per-kind isolation (a source that cannot sync must
// never hang the single-goroutine SourceReconciler).
func TestEndpoints_BoundedWaitReturnsNotReady(t *testing.T) {
	p := NewProvider(kubefake.NewSimpleClientset(), nil)
	p.buildWait = time.Nanosecond // force the timeout branch before the build can finish

	cfgs := BuildEffectiveConfigs([]sreportalv1alpha2.DNS{{Spec: sreportalv1alpha2.DNSSpec{
		Sources: sreportalv1alpha2.SourcesSpec{
			Service: &sreportalv1alpha2.ServiceSourceSpec{
				CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
			},
		},
	}}})

	_, err := p.Endpoints(context.Background(), KindService, cfgs[KindService])
	if !errors.Is(err, ErrSourceNotReady) {
		t.Fatalf("expected ErrSourceNotReady within bounded wait, got %v", err)
	}
}
