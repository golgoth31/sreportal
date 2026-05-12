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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// LookupPortalFeature fetches the Portal at namespace/name and reports whether the
// feature selected by isEnabled is active. A NotFound error is treated as "feature
// disabled, do not requeue" so deletions don't loop. Any other error is propagated
// so the workqueue retries — historically these were silently dropped, which made
// transient API failures look like the feature was enabled.
func LookupPortalFeature(
	ctx context.Context,
	c client.Client,
	namespace, name string,
	isEnabled func(*sreportalv1alpha1.PortalFeatures) bool,
) (bool, error) {
	var p sreportalv1alpha1.Portal
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &p); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get portal %s/%s: %w", namespace, name, err)
	}
	return isEnabled(p.Spec.Features), nil
}
