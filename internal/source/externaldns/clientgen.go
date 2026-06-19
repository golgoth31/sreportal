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

package externaldns

import (
	"fmt"
	"sync"

	openshift "github.com/openshift/client-go/route/clientset/versioned"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	externaldnssource "sigs.k8s.io/external-dns/source"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// clientGen implements external-dns' source.ClientGenerator, backed by the
// in-cluster rest.Config. The gateway-api route sources take a ClientGenerator
// rather than concrete clients; the kube and istio clients are shared with the
// Provider, while the gateway and dynamic clients are built lazily on first use.
// OpenShift is not supported (we never build OcpRoute sources).
type clientGen struct {
	restConfig *rest.Config
	kube       kubernetes.Interface
	istio      istioclient.Interface

	mu  sync.Mutex
	gw  gateway.Interface
	dyn dynamic.Interface
}

var _ externaldnssource.ClientGenerator = (*clientGen)(nil)

func (g *clientGen) KubeClient() (kubernetes.Interface, error) { return g.kube, nil }

func (g *clientGen) IstioClient() (istioclient.Interface, error) {
	if g.istio == nil {
		return nil, fmt.Errorf("istio client not configured")
	}
	return g.istio, nil
}

func (g *clientGen) RESTConfig() (*rest.Config, error) {
	if g.restConfig == nil {
		return nil, fmt.Errorf("rest config not configured")
	}
	return g.restConfig, nil
}

func (g *clientGen) GatewayClient() (gateway.Interface, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gw == nil {
		if g.restConfig == nil {
			return nil, fmt.Errorf("rest config not configured")
		}
		c, err := gateway.NewForConfig(g.restConfig)
		if err != nil {
			return nil, err
		}
		g.gw = c
	}
	return g.gw, nil
}

func (g *clientGen) DynamicKubernetesClient() (dynamic.Interface, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.dyn == nil {
		if g.restConfig == nil {
			return nil, fmt.Errorf("rest config not configured")
		}
		c, err := dynamic.NewForConfig(g.restConfig)
		if err != nil {
			return nil, err
		}
		g.dyn = c
	}
	return g.dyn, nil
}

func (g *clientGen) OpenShiftClient() (openshift.Interface, error) {
	return nil, fmt.Errorf("openshift client not supported")
}
