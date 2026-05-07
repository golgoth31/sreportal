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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
)

func TestChain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ImageRegistry Chain Suite")
}

// newScheme returns a runtime.Scheme with sreportal types registered.
func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

// newFakeClient returns a fake client with the sreportal scheme registered.
func newFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(objs...).WithStatusSubresource(&sreportalv1alpha1.ImageRegistry{}).Build()
}

// fakeRegistryClient is a hand-rolled fake for domainimageregistry.Client.
type fakeRegistryClient struct {
	tags map[string][]string // key: "host/repo"
	err  error
}

func (f *fakeRegistryClient) ListTags(_ context.Context, host, repo string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tags[host+"/"+repo], nil
}

var _ domainimageregistry.Client = (*fakeRegistryClient)(nil)

// Ensure fmt is used in test helpers.
var _ = fmt.Sprintf
