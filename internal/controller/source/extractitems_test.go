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

package source

import (
	"testing"

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestExtractItems_ValueSlice covers k8s core lists whose Items is []T.
func TestExtractItems_ValueSlice(t *testing.T) {
	list := &corev1.ServiceList{Items: []corev1.Service{
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	}}
	got, skipped := extractItems(list)
	if len(got) != 2 || skipped != 0 {
		t.Fatalf("want 2 items 0 skipped, got %d items %d skipped", len(got), skipped)
	}
	if got[0].GetName() != "a" || got[1].GetName() != "b" {
		t.Fatalf("unexpected names: %q, %q", got[0].GetName(), got[1].GetName())
	}
}

// TestExtractItems_PointerSlice covers generated clients whose Items is []*T
// (e.g. istio client-go). Before the fix, Addr()-ing a *T element yielded **T
// and panicked on the client.Object assertion.
func TestExtractItems_PointerSlice(t *testing.T) {
	list := &istionetworkingv1.GatewayList{Items: []*istionetworkingv1.Gateway{
		{ObjectMeta: metav1.ObjectMeta{Name: "gw"}},
	}}
	got, skipped := extractItems(list)
	if len(got) != 1 || skipped != 0 {
		t.Fatalf("want 1 item 0 skipped, got %d items %d skipped", len(got), skipped)
	}
	if got[0].GetName() != "gw" {
		t.Fatalf("unexpected name: %q", got[0].GetName())
	}
}

// TestExtractItems_NoItemsField returns nil rather than panicking.
func TestExtractItems_NoItemsField(t *testing.T) {
	if got, _ := extractItems(&corev1.ServiceList{}); len(got) != 0 {
		t.Fatalf("want empty for no items, got %d", len(got))
	}
}
