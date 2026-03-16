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

package registry

import (
	"fmt"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/rest"
)

// NewIstioClient creates an Istio clientset from a REST config.
// Shared by istio source builders to avoid duplicated creation logic.
func NewIstioClient(cfg *rest.Config) (istioclient.Interface, error) {
	ic, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create istio client: %w", err)
	}
	return ic, nil
}
