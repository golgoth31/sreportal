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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/imageregistry/chain"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("ValidateSpecHandler", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("spec validation",
		func(spec sreportalv1alpha1.ImageRegistrySpec, wantErr bool) {
			ir := &sreportalv1alpha1.ImageRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: tNsDefault},
				Spec:       spec,
			}
			c := newFakeClient(ir)
			handler := chain.NewValidateSpecHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{
				Resource: ir,
			}
			err := handler.Handle(ctx, rc)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, domainimageregistry.ErrInvalidSpec)).To(BeTrue())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("valid spec", sreportalv1alpha1.ImageRegistrySpec{
			Host:      tRegistryGhcr,
			PortalRef: tPortalMain,
			Namespace: tNsDefault,
		}, false),
		Entry("missing host", sreportalv1alpha1.ImageRegistrySpec{
			Host:      "",
			PortalRef: tPortalMain,
			Namespace: tNsDefault,
		}, true),
		Entry("missing portalRef", sreportalv1alpha1.ImageRegistrySpec{
			Host:      tRegistryGhcr,
			PortalRef: "",
			Namespace: tNsDefault,
		}, true),
		Entry("missing namespace", sreportalv1alpha1.ImageRegistrySpec{
			Host:      tRegistryGhcr,
			PortalRef: tPortalMain,
			Namespace: "",
		}, true),
	)
})
