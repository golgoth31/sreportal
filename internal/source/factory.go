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

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	externaldnssource "sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/annotations"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	openshift "github.com/openshift/client-go/route/clientset/versioned"

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
	// SourceTypeGatewayHTTPRoute indicates a Gateway API HTTPRoute source.
	SourceTypeGatewayHTTPRoute SourceType = "gateway-httproute"
	// SourceTypeGatewayGRPCRoute indicates a Gateway API GRPCRoute source.
	SourceTypeGatewayGRPCRoute SourceType = "gateway-grpcroute"
	// SourceTypeGatewayTLSRoute indicates a Gateway API TLSRoute source.
	SourceTypeGatewayTLSRoute SourceType = "gateway-tlsroute"
	// SourceTypeGatewayTCPRoute indicates a Gateway API TCPRoute source.
	SourceTypeGatewayTCPRoute SourceType = "gateway-tcproute"
	// SourceTypeGatewayUDPRoute indicates a Gateway API UDPRoute source.
	SourceTypeGatewayUDPRoute SourceType = "gateway-udproute"
)

// TypedSource pairs a Source with its type.
type TypedSource struct {
	Type   SourceType
	Source Source
}

// Factory creates external-dns sources from operator configuration.
type Factory struct {
	kubeClient    kubernetes.Interface
	restConfig    *rest.Config
	istioClient   istioclient.Interface
	gatewayClient gatewayclient.Interface
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

	// Build Gateway HTTPRoute source if enabled
	if cfg.Sources.GatewayHTTPRoute != nil && cfg.Sources.GatewayHTTPRoute.Enabled {
		log.Info("building gateway-httproute source", "namespace", cfg.Sources.GatewayHTTPRoute.Namespace)
		if err := f.ensureGatewayClient(); err != nil {
			log.Error(err, "failed to create gateway client")
			return nil, err
		}
		src, err := f.buildGatewayHTTPRouteSource(cfg.Sources.GatewayHTTPRoute)
		if err != nil {
			log.Error(err, "failed to build gateway-httproute source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeGatewayHTTPRoute,
			Source: src,
		})
		log.Info("gateway-httproute source built successfully")
	} else {
		log.Info("gateway-httproute source not enabled")
	}

	// Build Gateway GRPCRoute source if enabled
	if cfg.Sources.GatewayGRPCRoute != nil && cfg.Sources.GatewayGRPCRoute.Enabled {
		log.Info("building gateway-grpcroute source", "namespace", cfg.Sources.GatewayGRPCRoute.Namespace)
		if err := f.ensureGatewayClient(); err != nil {
			log.Error(err, "failed to create gateway client")
			return nil, err
		}
		src, err := f.buildGatewayGRPCRouteSource(cfg.Sources.GatewayGRPCRoute)
		if err != nil {
			log.Error(err, "failed to build gateway-grpcroute source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeGatewayGRPCRoute,
			Source: src,
		})
		log.Info("gateway-grpcroute source built successfully")
	} else {
		log.Info("gateway-grpcroute source not enabled")
	}

	// Build Gateway TLSRoute source if enabled
	if cfg.Sources.GatewayTLSRoute != nil && cfg.Sources.GatewayTLSRoute.Enabled {
		log.Info("building gateway-tlsroute source", "namespace", cfg.Sources.GatewayTLSRoute.Namespace)
		if err := f.ensureGatewayClient(); err != nil {
			log.Error(err, "failed to create gateway client")
			return nil, err
		}
		src, err := f.buildGatewayTLSRouteSource(cfg.Sources.GatewayTLSRoute)
		if err != nil {
			log.Error(err, "failed to build gateway-tlsroute source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeGatewayTLSRoute,
			Source: src,
		})
		log.Info("gateway-tlsroute source built successfully")
	} else {
		log.Info("gateway-tlsroute source not enabled")
	}

	// Build Gateway TCPRoute source if enabled
	if cfg.Sources.GatewayTCPRoute != nil && cfg.Sources.GatewayTCPRoute.Enabled {
		log.Info("building gateway-tcproute source", "namespace", cfg.Sources.GatewayTCPRoute.Namespace)
		if err := f.ensureGatewayClient(); err != nil {
			log.Error(err, "failed to create gateway client")
			return nil, err
		}
		src, err := f.buildGatewayTCPRouteSource(cfg.Sources.GatewayTCPRoute)
		if err != nil {
			log.Error(err, "failed to build gateway-tcproute source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeGatewayTCPRoute,
			Source: src,
		})
		log.Info("gateway-tcproute source built successfully")
	} else {
		log.Info("gateway-tcproute source not enabled")
	}

	// Build Gateway UDPRoute source if enabled
	if cfg.Sources.GatewayUDPRoute != nil && cfg.Sources.GatewayUDPRoute.Enabled {
		log.Info("building gateway-udproute source", "namespace", cfg.Sources.GatewayUDPRoute.Namespace)
		if err := f.ensureGatewayClient(); err != nil {
			log.Error(err, "failed to create gateway client")
			return nil, err
		}
		src, err := f.buildGatewayUDPRouteSource(cfg.Sources.GatewayUDPRoute)
		if err != nil {
			log.Error(err, "failed to build gateway-udproute source")
			return nil, err
		}
		typedSources = append(typedSources, TypedSource{
			Type:   SourceTypeGatewayUDPRoute,
			Source: src,
		})
		log.Info("gateway-udproute source built successfully")
	} else {
		log.Info("gateway-udproute source not enabled")
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
	if cfg.Sources.GatewayHTTPRoute != nil && cfg.Sources.GatewayHTTPRoute.Enabled {
		types = append(types, SourceTypeGatewayHTTPRoute)
	}
	if cfg.Sources.GatewayGRPCRoute != nil && cfg.Sources.GatewayGRPCRoute.Enabled {
		types = append(types, SourceTypeGatewayGRPCRoute)
	}
	if cfg.Sources.GatewayTLSRoute != nil && cfg.Sources.GatewayTLSRoute.Enabled {
		types = append(types, SourceTypeGatewayTLSRoute)
	}
	if cfg.Sources.GatewayTCPRoute != nil && cfg.Sources.GatewayTCPRoute.Enabled {
		types = append(types, SourceTypeGatewayTCPRoute)
	}
	if cfg.Sources.GatewayUDPRoute != nil && cfg.Sources.GatewayUDPRoute.Enabled {
		types = append(types, SourceTypeGatewayUDPRoute)
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

type sreportalClientGenerator struct {
	kubeClient    kubernetes.Interface
	gatewayClient gatewayclient.Interface
	istioClient   istioclient.Interface
}

func (g *sreportalClientGenerator) KubeClient() (kubernetes.Interface, error) {
	if g.kubeClient == nil {
		return nil, fmt.Errorf("kube client not initialized")
	}
	return g.kubeClient, nil
}

func (g *sreportalClientGenerator) GatewayClient() (gatewayclient.Interface, error) {
	if g.gatewayClient == nil {
		return nil, fmt.Errorf("gateway client not initialized")
	}
	return g.gatewayClient, nil
}

func (g *sreportalClientGenerator) IstioClient() (istioclient.Interface, error) {
	if g.istioClient == nil {
		return nil, fmt.Errorf("istio client not initialized")
	}
	return g.istioClient, nil
}

func (g *sreportalClientGenerator) DynamicKubernetesClient() (dynamic.Interface, error) {
	return nil, fmt.Errorf("dynamic client not supported")
}

func (g *sreportalClientGenerator) CloudFoundryClient(_, _, _ string) (*cfclient.Client, error) {
	return nil, fmt.Errorf("CloudFoundry not supported")
}

func (g *sreportalClientGenerator) OpenShiftClient() (openshift.Interface, error) {
	return nil, fmt.Errorf("OpenShift not supported")
}

func (f *Factory) ensureGatewayClient() error {
	if f.gatewayClient != nil {
		return nil
	}
	gc, err := gatewayclient.NewForConfig(f.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create gateway client: %w", err)
	}
	f.gatewayClient = gc
	return nil
}

func (f *Factory) buildGatewayHTTPRouteSource(cfg *config.GatewayHTTPRouteConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	clientGen := &sreportalClientGenerator{
		kubeClient:    f.kubeClient,
		gatewayClient: f.gatewayClient,
	}

	sourceCfg := &externaldnssource.Config{
		Namespace:                cfg.Namespace,
		AnnotationFilter:         cfg.AnnotationFilter,
		LabelFilter:              labelSelector,
		FQDNTemplate:             cfg.FQDNTemplate,
		CombineFQDNAndAnnotation: cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation: cfg.IgnoreHostnameAnnotation,
		GatewayName:              cfg.GatewayName,
		GatewayNamespace:         cfg.GatewayNamespace,
		GatewayLabelFilter:       cfg.GatewayLabelFilter,
	}

	return externaldnssource.NewGatewayHTTPRouteSource(clientGen, sourceCfg)
}

func (f *Factory) buildGatewayGRPCRouteSource(cfg *config.GatewayGRPCRouteConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	clientGen := &sreportalClientGenerator{
		kubeClient:    f.kubeClient,
		gatewayClient: f.gatewayClient,
	}

	sourceCfg := &externaldnssource.Config{
		Namespace:                cfg.Namespace,
		AnnotationFilter:         cfg.AnnotationFilter,
		LabelFilter:              labelSelector,
		FQDNTemplate:             cfg.FQDNTemplate,
		CombineFQDNAndAnnotation: cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation: cfg.IgnoreHostnameAnnotation,
		GatewayName:              cfg.GatewayName,
		GatewayNamespace:         cfg.GatewayNamespace,
		GatewayLabelFilter:       cfg.GatewayLabelFilter,
	}

	return externaldnssource.NewGatewayGRPCRouteSource(clientGen, sourceCfg)
}

func (f *Factory) buildGatewayTLSRouteSource(cfg *config.GatewayTLSRouteConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	clientGen := &sreportalClientGenerator{
		kubeClient:    f.kubeClient,
		gatewayClient: f.gatewayClient,
	}

	sourceCfg := &externaldnssource.Config{
		Namespace:                cfg.Namespace,
		AnnotationFilter:         cfg.AnnotationFilter,
		LabelFilter:              labelSelector,
		FQDNTemplate:             cfg.FQDNTemplate,
		CombineFQDNAndAnnotation: cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation: cfg.IgnoreHostnameAnnotation,
		GatewayName:              cfg.GatewayName,
		GatewayNamespace:         cfg.GatewayNamespace,
		GatewayLabelFilter:       cfg.GatewayLabelFilter,
	}

	return externaldnssource.NewGatewayTLSRouteSource(clientGen, sourceCfg)
}

func (f *Factory) buildGatewayTCPRouteSource(cfg *config.GatewayTCPRouteConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	clientGen := &sreportalClientGenerator{
		kubeClient:    f.kubeClient,
		gatewayClient: f.gatewayClient,
	}

	sourceCfg := &externaldnssource.Config{
		Namespace:                cfg.Namespace,
		AnnotationFilter:         cfg.AnnotationFilter,
		LabelFilter:              labelSelector,
		FQDNTemplate:             cfg.FQDNTemplate,
		CombineFQDNAndAnnotation: cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation: cfg.IgnoreHostnameAnnotation,
		GatewayName:              cfg.GatewayName,
		GatewayNamespace:         cfg.GatewayNamespace,
		GatewayLabelFilter:       cfg.GatewayLabelFilter,
	}

	return externaldnssource.NewGatewayTCPRouteSource(clientGen, sourceCfg)
}

func (f *Factory) buildGatewayUDPRouteSource(cfg *config.GatewayUDPRouteConfig) (Source, error) {
	labelSelector, err := parseLabelSelector(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}

	clientGen := &sreportalClientGenerator{
		kubeClient:    f.kubeClient,
		gatewayClient: f.gatewayClient,
	}

	sourceCfg := &externaldnssource.Config{
		Namespace:                cfg.Namespace,
		AnnotationFilter:         cfg.AnnotationFilter,
		LabelFilter:              labelSelector,
		FQDNTemplate:             cfg.FQDNTemplate,
		CombineFQDNAndAnnotation: cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation: cfg.IgnoreHostnameAnnotation,
		GatewayName:              cfg.GatewayName,
		GatewayNamespace:         cfg.GatewayNamespace,
		GatewayLabelFilter:       cfg.GatewayLabelFilter,
	}

	return externaldnssource.NewGatewayUDPRouteSource(clientGen, sourceCfg)
}
