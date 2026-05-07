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

package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
)

// nolint:unused
var imageregistrylog = log.Default().WithName("imageregistry-resource")

// SetupImageRegistryWebhookWithManager registers the validation webhook for ImageRegistry.
func SetupImageRegistryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.ImageRegistry{}).
		WithValidator(&ImageRegistryCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-imageregistry,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=imageregistries,verbs=create;update,versions=v1alpha1,name=vimageregistry-v1alpha1.kb.io,admissionReviewVersions=v1

// ImageRegistryCustomValidator validates the ImageRegistry resource. The CR
// itself is controller-managed in v1, but the webhook still enforces basic
// invariants (host parses, ChangeType ↔ OriginalImage coherence) so a
// malformed CR cannot poison downstream readers.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen
// from generating DeepCopy methods, as this struct is used only for temporary
// operations and does not need to be deeply copied.
type ImageRegistryCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon creation", "name", obj.GetName())
	return nil, validateImageRegistry(obj)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateUpdate(_ context.Context, _, newObj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon update", "name", newObj.GetName())
	return nil, validateImageRegistry(newObj)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon deletion", "name", obj.GetName())
	return nil, nil
}

// validateImageRegistry runs the full set of invariant checks documented in
// plan §3.4.
func validateImageRegistry(obj *sreportalv1alpha1.ImageRegistry) error {
	if obj.Spec.Host == "" {
		return fmt.Errorf("spec.host is required")
	}
	if _, err := name.NewRegistry(obj.Spec.Host); err != nil {
		return fmt.Errorf("spec.host %q is not a valid registry: %w", obj.Spec.Host, err)
	}
	if err := assertHostNotForbidden(obj.Spec.Host); err != nil {
		return fmt.Errorf("spec.host: %w", err)
	}
	if obj.Spec.PortalRef == "" {
		return fmt.Errorf("spec.portalRef is required")
	}
	if obj.Spec.Namespace == "" {
		return fmt.Errorf("spec.namespace is required")
	}

	for i, e := range obj.Spec.Images {
		if e.Key == "" {
			return fmt.Errorf("spec.images[%d].key is required", i)
		}
		if e.MutatedImage == "" {
			return fmt.Errorf("spec.images[%d].mutatedImage is required", i)
		}
		if err := assertImageHostMatches(e.MutatedImage, obj.Spec.Host); err != nil {
			return fmt.Errorf("spec.images[%d].mutatedImage: %w", i, err)
		}
		switch e.ChangeType {
		case "none", "mutated":
			if e.OriginalImage == "" {
				return fmt.Errorf("spec.images[%d]: changeType=%q requires originalImage", i, e.ChangeType)
			}
			if err := assertImageHostMatches(e.OriginalImage, obj.Spec.Host); err != nil {
				return fmt.Errorf("spec.images[%d].originalImage: %w", i, err)
			}
		case "injected":
			if e.OriginalImage != "" {
				return fmt.Errorf("spec.images[%d]: changeType=injected forbids originalImage", i)
			}
		default:
			return fmt.Errorf("spec.images[%d].changeType %q must be one of none|mutated|injected", i, e.ChangeType)
		}
	}
	return nil
}

// assertHostNotForbidden rejects registry hosts pointing to private,
// loopback, link-local or in-cluster networks. Goal: prevent the operator
// from being weaponised as an SSRF vector against the cluster's metadata
// service (169.254.169.254) or internal services.
//
// The check is best-effort — it inspects the literal host string only and
// does not perform DNS resolution at admission time (DNS results are racy
// and the validator runs in the API server, not in cluster network).
func assertHostNotForbidden(host string) error {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	if hostname == "" {
		return fmt.Errorf("host is empty")
	}
	if hostname == "localhost" {
		return fmt.Errorf("host %q is not allowed (loopback)", host)
	}
	if strings.HasSuffix(hostname, ".localhost") {
		return fmt.Errorf("host %q is not allowed (loopback domain)", host)
	}
	// Common in-cluster suffixes — services exposed via kube-dns are not a
	// valid registry source for an external image lookup.
	for _, suffix := range []string{".cluster.local", ".svc", ".svc.cluster.local"} {
		if strings.HasSuffix(hostname, suffix) {
			return fmt.Errorf("host %q is not allowed (in-cluster DNS)", host)
		}
	}
	// Numeric IP literal — apply RFC1918 / loopback / link-local denylist.
	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("host %q is not allowed (private, loopback, link-local or multicast IP)", host)
		}
	}
	return nil
}

// assertImageHostMatches verifies the registry host parsed from `image`
// matches `expectedHost`. Prevents a CR pointing at a Spec.Host=ghcr.io while
// listing docker.io/library/nginx images: such a CR would silently ask the
// wrong registry for tags and never resolve.
func assertImageHostMatches(image, expectedHost string) error {
	ref, err := name.ParseReference(image)
	if err != nil {
		return fmt.Errorf("parse %q: %w", image, err)
	}
	got := ref.Context().RegistryStr()
	if !registryHostsEqual(got, expectedHost) {
		return fmt.Errorf("image registry %q does not match spec.host %q", got, expectedHost)
	}
	return nil
}

// registryHostsEqual treats Docker Hub aliases as equivalent because
// go-containerregistry canonicalises "docker.io" → "index.docker.io".
func registryHostsEqual(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a == b {
		return true
	}
	dockerAliases := map[string]struct{}{
		"docker.io":            {},
		"index.docker.io":      {},
		"registry-1.docker.io": {},
	}
	_, aDocker := dockerAliases[a]
	_, bDocker := dockerAliases[b]
	return aDocker && bDocker
}
