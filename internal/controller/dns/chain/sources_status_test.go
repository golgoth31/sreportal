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

package dns_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type fakeConflicts struct{ events []domaindns.ConflictEvent }

func (f fakeConflicts) Conflicts(string, string) []domaindns.ConflictEvent { return f.events }

func TestSourcesStatus_NoConflicts(t *testing.T) {
	dns := &sreportalv1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "n"}}
	h := &dnschain.SourcesStatusHandler{Conflicts: fakeConflicts{}}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{Resource: dns}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, metav1.ConditionTrue, conditionStatus(dns, "SourcesReady"))
	require.Equal(t, metav1.ConditionFalse, conditionStatus(dns, "TargetsConflict"))
}

func TestSourcesStatus_WithConflicts(t *testing.T) {
	dns := &sreportalv1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "n"}}
	h := &dnschain.SourcesStatusHandler{Conflicts: fakeConflicts{events: []domaindns.ConflictEvent{{LoserRecord: "n/d"}}}}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{Resource: dns}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, metav1.ConditionTrue, conditionStatus(dns, "TargetsConflict"))
}

func TestSourcesStatus_NilConflictsReader(t *testing.T) {
	dns := &sreportalv1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "n"}}
	h := &dnschain.SourcesStatusHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{Resource: dns}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, metav1.ConditionTrue, conditionStatus(dns, "SourcesReady"))
	require.Equal(t, metav1.ConditionFalse, conditionStatus(dns, "TargetsConflict"))
}

func conditionStatus(dns *sreportalv1alpha2.DNS, t string) metav1.ConditionStatus {
	for _, c := range dns.Status.Conditions {
		if c.Type == t {
			return c.Status
		}
	}
	return ""
}
