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
	"context"
	"fmt"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	externaldnssource "sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/annotations"

	"github.com/golgoth31/sreportal/internal/config"
)

// Source is an alias for the external-dns source.Source interface.
type Source = externaldnssource.Source

// SourceType represents the type of external-dns source.
type SourceType string

const (
	// SourceTypeService indicates a Kubernetes Service source.
	SourceTypeService SourceType = "service"
	// SourceTypeIngress indicates a Kubernetes Ingress source.
	SourceTypeIngress SourceType = "ingress"
	// SourceTypeDNSEndpoint indicates an external-dns DNSEndpoint CRD source.
	SourceTypeDNSEndpoint SourceType = "dnsendpoint"
	// SourceTypeIstioGateway indicates an Istio Gateway source.
	SourceTypeIstioGateway SourceType = "istio-gateway"
	// SourceTypeIstioVirtualService indicates an Istio VirtualService source.
	SourceTypeIstioVirtualService SourceType = "istio-virtualservice"
)

// TypedSource pairs a Source with its type.
type TypedSource struct {
	Type   SourceType
	Source Source
}

// Factory creates external-dns sources from operator configuration.
type Factory struct {
	kubeClient  kubernetes.Interface
	restConfig  *rest.Config
	istioClient istioclient.Interface
}

// NewFactory creates a new source Factory.
// It initialises external-dns annotation keys before any source is built.
// In external-dns v0.20.0 the annotation keys (HostnameKey, etc.) are empty
// strings until SetAnnotationPrefix is called; doing it here avoids the
// implicit init() side-effect that was previously in this package.
func NewFactory(kubeClient kubernetes.Interface, restConfig *rest.Config) *Factory {
	annotations.SetAnnotationPrefix("external-dns.alpha.kubernetes.io/")
	return &Factory{
		kubeClient: kubeClient,
		restConfig: restConfig,
	}
}

// BuildTypedSources creates external-dns sources with type information.
// Each source is returned individually with its type, enabling per-source DNSRecord management.
func (f *Factory) BuildTypedSources(ctx context.Context, cfg *config.OperatorConfig) ([]TypedSource, error) {
	log := ctrl.Log.WithName("source-factory")
	var typedSources []TypedSource

	// Build Service source if enabled
	if cfg.Sources.Service != nil && cfg.Sources.Service.Enabled {
		log.Info("building service source", "namespace", cfg.Sources.Service.Namespace,
			"annotationFilter", cfg.Sources.Service.AnnotationFilter,
			"serviceTypeFilter", cfg.Sources.Service.ServiceTypeFilter)
		src, err := f.buildServiceSource(ctx, cfg.Sources.Service)
		if err != nil {
			log.Error(err, "failed to build service source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeService,
			Source: src,
		})
		log.Info("service source built successfully")
	} else {
		log.Info("service source not enabled", "serviceConfig", cfg.Sources.Service)
	}

	// Build Ingress source if enabled
	if cfg.Sources.Ingress != nil && cfg.Sources.Ingress.Enabled {
		log.Info("building ingress source")
		src, err := f.buildIngressSource(ctx, cfg.Sources.Ingress)
		if err != nil {
			log.Error(err, "failed to build ingress source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeIngress,
			Source: src,
		})
		log.Info("ingress source built successfully")
	} else {
		log.Info("ingress source not enabled")
	}

	// Build DNSEndpoint CRD source if enabled
	if cfg.Sources.DNSEndpoint != nil && cfg.Sources.DNSEndpoint.Enabled {
		log.Info("building dnsendpoint source")
		src, err := f.buildDNSEndpointSource(cfg.Sources.DNSEndpoint)
		if err != nil {
			log.Error(err, "failed to build dnsendpoint source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeDNSEndpoint,
			Source: src,
		})
		log.Info("dnsendpoint source built successfully")
	} else {
		log.Info("dnsendpoint source not enabled")
	}

	// Build Istio Gateway source if enabled
	if cfg.Sources.IstioGateway != nil && cfg.Sources.IstioGateway.Enabled {
		log.Info("building istio-gateway source", "namespace", cfg.Sources.IstioGateway.Namespace)
		if err := f.ensureIstioClient(); err != nil {
			log.Error(err, "failed to create istio client")
			return nil, err
		}
		src, err := f.buildIstioGatewaySource(ctx, cfg.Sources.IstioGateway)
		if err != nil {
			log.Error(err, "failed to build istio-gateway source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeIstioGateway,
			Source: src,
		})
		log.Info("istio-gateway source built successfully")
	} else {
		log.Info("istio-gateway source not enabled")
	}

	// Build Istio VirtualService source if enabled
	if cfg.Sources.IstioVirtualService != nil && cfg.Sources.IstioVirtualService.Enabled {
		log.Info("building istio-virtualservice source", "namespace", cfg.Sources.IstioVirtualService.Namespace)
		if err := f.ensureIstioClient(); err != nil {
			log.Error(err, "failed to create istio client")
			return nil, err
		}
		src, err := f.buildIstioVirtualServiceSource(ctx, cfg.Sources.IstioVirtualService)
		if err != nil {
			log.Error(err, "failed to build istio-virtualservice source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeIstioVirtualService,
			Source: src,
		})
		log.Info("istio-virtualservice source built successfully")
	} else {
		log.Info("istio-virtualservice source not enabled")
	}

	log.Info("sources built", "count", len(typedSources))

	return typedSources, nil
}

// EnabledSourceTypes returns the list of enabled source types from configuration.
func EnabledSourceTypes(cfg *config.OperatorConfig) []SourceType {
	var types []SourceType

	if cfg.Sources.Service != nil && cfg.Sources.Service.Enabled {
		types = append(types, SourceTypeService)
	}
	if cfg.Sources.Ingress != nil && cfg.Sources.Ingress.Enabled {
		types = append(types, SourceTypeIngress)
	}
	if cfg.Sources.DNSEndpoint != nil && cfg.Sources.DNSEndpoint.Enabled {
		types = append(types, SourceTypeDNSEndpoint)
	}
	if cfg.Sources.IstioGateway != nil && cfg.Sources.IstioGateway.Enabled {
		types = append(types, SourceTypeIstioGateway)
	}
	if cfg.Sources.IstioVirtualService != nil && cfg.Sources.IstioVirtualService.Enabled {
		types = append(types, SourceTypeIstioVirtualService)
	}

	return types
}

func (f *Factory) buildServiceSource(ctx context.Context, cfg *config.ServiceConfig) (Source, error) {
	log := ctrl.Log.WithName("source-factory")
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	log.Info("building service source with parameters",
		"namespace", cfg.Namespace,
		"annotationFilter", cfg.AnnotationFilter,
		"fqdnTemplate", cfg.FQDNTemplate,
		"combineFQDNAndAnnotation", cfg.CombineFQDNAndAnnotation,
		"publishInternal", cfg.PublishInternal,
		"publishHostIP", cfg.PublishHostIP,
		"serviceTypeFilter", cfg.ServiceTypeFilter,
		"ignoreHostnameAnnotation", cfg.IgnoreHostnameAnnotation,
		"labelSelector", labelSelector.String(),
		"resolveLoadBalancerHostname", cfg.ResolveLoadBalancerHostname,
	)

	return externaldnssource.NewServiceSource(
		ctx,
		f.kubeClient,
		cfg.Namespace,
		cfg.AnnotationFilter,
		cfg.FQDNTemplate,
		cfg.CombineFQDNAndAnnotation,
		"", // compatibility mode (empty = default)
		cfg.PublishInternal,
		cfg.PublishHostIP,
		false, // alwaysPublishNotReadyAddresses
		cfg.ServiceTypeFilter,
		cfg.IgnoreHostnameAnnotation,
		labelSelector,
		cfg.ResolveLoadBalancerHostname,
		false, // listenEndpointEvents
		false, // exposeInternalIPv6
	)
}

func (f *Factory) buildIngressSource(ctx context.Context, cfg *config.IngressConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	return externaldnssource.NewIngressSource(
		ctx,
		f.kubeClient,
		cfg.Namespace,
		cfg.AnnotationFilter,
		cfg.FQDNTemplate,
		cfg.CombineFQDNAndAnnotation,
		cfg.IgnoreHostnameAnnotation,
		cfg.IgnoreIngressTLSSpec,
		cfg.IgnoreIngressRulesSpec,
		labelSelector,
		cfg.IngressClassNames,
	)
}

func (f *Factory) buildDNSEndpointSource(cfg *config.DNSEndpointConfig) (Source, error) {
	// Create a REST client for the DNSEndpoint CRD
	crdClient, scheme, err := f.createCRDClient()
	if err != nil {
		return nil, err
	}

	return externaldnssource.NewCRDSource(
		crdClient,
		cfg.Namespace,
		"DNSEndpoint",
		"", // annotationFilter (empty = no filter)
		labels.Everything(),
		scheme,
		true, // startInformer
	)
}

func (f *Factory) ensureIstioClient() error {
	if f.istioClient != nil {
		return nil
	}
	ic, err := istioclient.NewForConfig(f.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create istio client: %w", err)
	}
	f.istioClient = ic
	return nil
}

func (f *Factory) buildIstioGatewaySource(ctx context.Context, cfg *config.IstioGatewayConfig) (Source, error) {
	return externaldnssource.NewIstioGatewaySource(
		ctx,
		f.kubeClient,
		f.istioClient,
		cfg.Namespace,
		cfg.AnnotationFilter,
		cfg.FQDNTemplate,
		cfg.CombineFQDNAndAnnotation,
		cfg.IgnoreHostnameAnnotation,
	)
}

func (f *Factory) buildIstioVirtualServiceSource(ctx context.Context, cfg *config.IstioVirtualServiceConfig) (Source, error) {
	return externaldnssource.NewIstioVirtualServiceSource(
		ctx,
		f.kubeClient,
		f.istioClient,
		cfg.Namespace,
		cfg.AnnotationFilter,
		cfg.FQDNTemplate,
		cfg.CombineFQDNAndAnnotation,
		cfg.IgnoreHostnameAnnotation,
	)
}

func (f *Factory) createCRDClient() (rest.Interface, *runtime.Scheme, error) {
	return externaldnssource.NewCRDClientForAPIVersionKind(
		f.kubeClient,
		"", // kubeConfig (empty = use in-cluster)
		"", // apiServerURL (empty = use in-cluster)
		"externaldns.k8s.io/v1alpha1",
		"DNSEndpoint",
	)
}

func parseLabelSelector(selector string) (labels.Selector, error) {
	if selector == "" {
		return labels.Everything(), nil
	}
	return labels.Parse(selector)
}
