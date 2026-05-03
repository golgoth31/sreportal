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
	"errors"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	openshift "github.com/openshift/client-go/route/clientset/versioned"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	externaldnssource "sigs.k8s.io/external-dns/source"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// errUnsupportedClient is returned for client types not supported by the adapter.
var errUnsupportedClient = errors.New("unsupported client type in gateway source adapter")

// Verify interface compliance at compile time.
var _ externaldnssource.ClientGenerator = (*GatewayClientGenerator)(nil)

// GatewayClientGenerator adapts Deps to the external-dns ClientGenerator interface.
// Only KubeClient and GatewayClient are supported; all other methods return errors
// because gateway route sources never call them.
type GatewayClientGenerator struct {
	kubeClient    kubernetes.Interface
	gatewayClient gateway.Interface
}

// NewGatewayClientGenerator creates a ClientGenerator from Deps and a pre-built Gateway API client.
func NewGatewayClientGenerator(deps Deps, gwClient gateway.Interface) *GatewayClientGenerator {
	return &GatewayClientGenerator{
		kubeClient:    deps.KubeClient,
		gatewayClient: gwClient,
	}
}

// KubeClient returns the standard Kubernetes client.
func (g *GatewayClientGenerator) KubeClient() (kubernetes.Interface, error) {
	return g.kubeClient, nil
}

// GatewayClient returns the Gateway API client.
func (g *GatewayClientGenerator) GatewayClient() (gateway.Interface, error) {
	return g.gatewayClient, nil
}

// IstioClient is not supported by this adapter.
func (g *GatewayClientGenerator) IstioClient() (istioclient.Interface, error) {
	return nil, errUnsupportedClient
}

// CloudFoundryClient is not supported by this adapter.
func (g *GatewayClientGenerator) CloudFoundryClient(_, _, _ string) (*cfclient.Client, error) {
	return nil, errUnsupportedClient
}

// DynamicKubernetesClient is not supported by this adapter.
func (g *GatewayClientGenerator) DynamicKubernetesClient() (dynamic.Interface, error) {
	return nil, errUnsupportedClient
}

// OpenShiftClient is not supported by this adapter.
func (g *GatewayClientGenerator) OpenShiftClient() (openshift.Interface, error) {
	return nil, errUnsupportedClient
}

// RESTConfig is not supported by this adapter.
func (g *GatewayClientGenerator) RESTConfig() (*rest.Config, error) {
	return nil, errUnsupportedClient
}
