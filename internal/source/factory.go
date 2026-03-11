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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/golgoth31/sreportal/internal/config"
)

// Factory assembles external-dns sources from registered builders.
type Factory struct {
	deps     Deps
	builders []Builder
}

// NewFactory creates a Factory with the given builders.
func NewFactory(kubeClient kubernetes.Interface, restConfig *rest.Config, builders []Builder) *Factory {
	return &Factory{
		deps: Deps{
			KubeClient: kubeClient,
			RestConfig: restConfig,
		},
		builders: builders,
	}
}

// BuildTypedSources creates external-dns sources for all enabled types.
// Sources are built in the order of registered builders.
func (f *Factory) BuildTypedSources(ctx context.Context, cfg *config.OperatorConfig) ([]TypedSource, error) {
	log := ctrl.Log.WithName("source-factory")
	var typedSources []TypedSource

	for _, b := range f.builders {
		if !b.Enabled(cfg) {
			log.V(1).Info("source not enabled", "type", b.Type())
			continue
		}
		log.Info("building source", "type", b.Type())
		src, err := b.Build(ctx, f.deps, cfg)
		if err != nil {
			return nil, fmt.Errorf("build %s source: %w", b.Type(), err)
		}
		typedSources = append(typedSources, TypedSource{Type: b.Type(), Source: src})
		log.Info("source built successfully", "type", b.Type())
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
func (f *Factory) EnabledSourceTypes(cfg *config.OperatorConfig) []SourceType {
	var types []SourceType
	for _, b := range f.builders {
		if b.Enabled(cfg) {
			types = append(types, b.Type())
		}
	}
	return types
}

// GVRForSourceType resolves the GroupVersionResource for a given source type
// by looking up the registered builder. Returns false if no builder matches
// or the source type does not support annotation enrichment.
func (f *Factory) GVRForSourceType(sourceType SourceType) (schema.GroupVersionResource, bool) {
	for _, b := range f.builders {
		if b.Type() == sourceType {
			return b.GVR()
		}
	}
	return schema.GroupVersionResource{}, false
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
