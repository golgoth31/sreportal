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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// LoadDNSConfigHandler loads the DNS CR referenced by the DNSRecord and
// populates the shared ChainData with its group mapping and reconciliation
// configuration.
type LoadDNSConfigHandler struct {
	client client.Client
}

// NewLoadDNSConfigHandler constructs a LoadDNSConfigHandler.
func NewLoadDNSConfigHandler(c client.Client) *LoadDNSConfigHandler {
	return &LoadDNSConfigHandler{client: c}
}

// Handle fetches the DNS CR referenced by the record's spec.portalRef and
// copies the relevant configuration into rc.Data.
func (h *LoadDNSConfigHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	record := rc.Resource
	var dns v1alpha2.DNS
	if err := h.client.Get(ctx, client.ObjectKey{
		Name:      record.Spec.PortalRef,
		Namespace: record.Namespace,
	}, &dns); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// DNS CR absent — short-circuit so downstream handlers don't run
			// with default config. The DNS watch in SetupWithManager will
			// re-enqueue this DNSRecord when the DNS CR appears.
			return reconciler.ErrShortCircuit
		}
		return fmt.Errorf("load DNS config for portal %q: %w", record.Spec.PortalRef, err)
	}
	rc.Data.GroupMapping = &dns.Spec.GroupMapping
	rc.Data.DisableDNSCheck = dns.Spec.Reconciliation.DisableDNSCheck
	return nil
}
