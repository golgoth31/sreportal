// Package statusutil provides shared helpers for status page controllers.
package statusutil

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConditionAccessor is implemented by CRD types that expose status conditions.
type ConditionAccessor interface {
	client.Object
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
}

// SetConditionAndPatch sets a condition on the object and patches the status subresource.
// It only patches if the condition actually changed.
func SetConditionAndPatch(ctx context.Context, c client.Client, obj ConditionAccessor, condType string, status metav1.ConditionStatus, reason, message string) error {
	conditions := obj.GetConditions()
	changed := false
	found := false

	for i, cond := range conditions {
		if cond.Type == condType {
			found = true
			if cond.Status != status || cond.Reason != reason || cond.Message != message {
				conditions[i] = metav1.Condition{
					Type:               condType,
					Status:             status,
					Reason:             reason,
					Message:            message,
					LastTransitionTime: metav1.Now(),
				}
				changed = true
			}
			break
		}
	}

	if !found {
		conditions = append(conditions, metav1.Condition{
			Type:               condType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
		})
		changed = true
	}

	if !changed {
		return nil
	}

	obj.SetConditions(conditions)
	if err := c.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("update %s status condition: %w", obj.GetObjectKind().GroupVersionKind().Kind, err)
	}

	return nil
}
