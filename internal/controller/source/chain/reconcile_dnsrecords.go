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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// ReconcileDNSRecordsHandler creates or updates DNSRecord CRs for each
// (portal, sourceType) pair in the collected endpoints.
type ReconcileDNSRecordsHandler struct {
	client client.Client
}

// NewReconcileDNSRecordsHandler creates a new ReconcileDNSRecordsHandler.
func NewReconcileDNSRecordsHandler(c client.Client) *ReconcileDNSRecordsHandler {
	return &ReconcileDNSRecordsHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ReconcileDNSRecordsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	if rc.Data.Index == nil {
		return nil
	}

	logger := log.FromContext(ctx).WithName("reconcile-dnsrecords")

	for key, endpoints := range rc.Data.EndpointsByPortalSource {
		portal := rc.Data.Index.ByName[key.PortalName]
		if portal == nil || portal.Spec.Remote != nil {
			continue
		}
		if err := h.reconcileDNSRecord(ctx, portal, key.SourceType, endpoints); err != nil {
			logger.Error(err, "failed to reconcile DNSRecord",
				"portal", key.PortalName, "sourceType", key.SourceType)
		}
	}

	return nil
}

func (h *ReconcileDNSRecordsHandler) reconcileDNSRecord(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	sourceType registry.SourceType,
	endpoints []*endpoint.Endpoint,
) error {
	logger := log.FromContext(ctx).WithName("reconcile-dnsrecords")

	name := fmt.Sprintf("%s-%s", portal.Name, sourceType)
	now := metav1.Now()

	dnsRecord := &sreportalv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: portal.Namespace,
		},
	}

	result, err := createOrUpdateSpec(ctx, h.client, dnsRecord, func() error {
		dnsRecord.Spec.SourceType = string(sourceType)
		dnsRecord.Spec.PortalRef = portal.Name
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to create/update DNSRecord",
			"name", name, "portal", portal.Name)
		return err
	}

	logger.V(1).Info("DNSRecord reconciled", "name", name, "result", result)

	newHash := adapter.EndpointsHash(endpoints)
	endpointStatus := adapter.ToEndpointStatus(endpoints)

	statusRetryBackoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
	}

	var skipped bool

	err = retry.OnError(statusRetryBackoff, func(err error) bool {
		return apierrors.IsNotFound(err) || apierrors.IsConflict(err)
	}, func() error {
		dnsRecordKey := client.ObjectKey{Namespace: portal.Namespace, Name: name}
		if err := h.client.Get(ctx, dnsRecordKey, dnsRecord); err != nil {
			return err
		}

		if dnsRecord.Status.EndpointsHash == newHash {
			skipped = true
			return nil
		}

		dnsRecord.Status.Endpoints = endpointStatus
		dnsRecord.Status.EndpointsHash = newHash
		dnsRecord.Status.LastReconcileTime = &now
		meta.SetStatusCondition(&dnsRecord.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "EndpointsCollected",
			Message:            fmt.Sprintf("Collected %d endpoints from %s source", len(endpoints), sourceType),
			LastTransitionTime: now,
		})

		return h.client.Status().Update(ctx, dnsRecord)
	})
	if err != nil {
		logger.Error(err, "failed to update DNSRecord status",
			"name", name, "portal", portal.Name)
		return err
	}

	if skipped {
		logger.V(1).Info("endpoints unchanged, skipped status update",
			"name", name, "hash", newHash)
		metrics.SourceSkippedUpdates.WithLabelValues(string(sourceType)).Inc()
	}

	return nil
}

// createOrUpdateSpec creates or updates only the spec of a DNSRecord with conflict retry.
func createOrUpdateSpec(ctx context.Context, c client.Client, obj *sreportalv1alpha1.DNSRecord, mutate func() error) (string, error) {
	key := client.ObjectKeyFromObject(obj)
	var result string

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing := &sreportalv1alpha1.DNSRecord{}
		err := c.Get(ctx, key, existing)
		if apierrors.IsNotFound(err) {
			if err := mutate(); err != nil {
				return err
			}
			result = "created"
			return c.Create(ctx, obj)
		}
		if err != nil {
			return err
		}

		obj.ObjectMeta = existing.ObjectMeta
		if err := mutate(); err != nil {
			return err
		}
		result = "updated"
		return c.Update(ctx, obj)
	})

	return result, err
}
