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

	"k8s.io/client-go/rest"
	externaldnssource "sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/template"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"github.com/golgoth31/sreportal/internal/config"
)

// NewGatewayClient creates a Gateway API clientset from a REST config.
// Shared by gateway route source builders to avoid duplicated creation logic.
func NewGatewayClient(cfg *rest.Config) (gateway.Interface, error) {
	gc, err := gateway.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create gateway-api client: %w", err)
	}
	return gc, nil
}

// NewGatewaySourceConfig converts a GatewayRouteConfig to the external-dns source.Config
// used by the Gateway API route constructors.
func NewGatewaySourceConfig(gc *config.GatewayRouteConfig) (*externaldnssource.Config, error) {
	labelSelector, err := ParseLabelSelector(gc.LabelFilter)
	if err != nil {
		return nil, err
	}
	tmpls, err := template.NewEngine(gc.FQDNTemplate, "", "", gc.CombineFQDNAndAnnotation)
	if err != nil {
		return nil, fmt.Errorf("build template engine: %w", err)
	}
	return &externaldnssource.Config{
		Namespace:                gc.Namespace,
		AnnotationFilter:         gc.AnnotationFilter,
		LabelFilter:              labelSelector,
		TemplateEngine:           tmpls,
		IgnoreHostnameAnnotation: gc.IgnoreHostnameAnnotation,
		GatewayName:              gc.GatewayName,
		GatewayNamespace:         gc.GatewayNamespace,
		GatewayLabelFilter:       gc.GatewayLabelFilter,
	}, nil
}
