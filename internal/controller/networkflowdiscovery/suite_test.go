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

package networkflowdiscovery_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestNetworkFlowDiscoveryHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetworkFlowDiscovery Handler Suite")
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	_ = networkingv1.AddToScheme(s)

	return s
}
