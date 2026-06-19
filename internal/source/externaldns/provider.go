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
	"context"
	"fmt"
	"sync"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/external-dns/endpoint"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

type builtSource struct {
	src    externaldnssource.Source
	hash   string
	cancel context.CancelFunc
}

// The native external-dns ServiceSource builds EndpointSlice + Pod informers
// when ClusterIP/NodePort are in scope, and a Node informer for NodePort —
// beyond the services/pods the hand-rolled resolvers needed. Grant those reads.
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Provider builds and memoizes native external-dns sources, one per kind.
//
// Each source owns a long-lived informer running on a child of the manager
// context; a source is rebuilt (and its old informer cancelled) only when its
// effective-config hash changes — so steady-state cost is one informer per kind
// regardless of the number of DNS CRs (Option B). Construction blocks until the
// informer cache syncs, so endpoints are always read from a synced cache.
type Provider struct {
	kube  kubernetes.Interface
	istio istioclient.Interface

	mu    sync.Mutex
	built map[registry.SourceType]builtSource
}

// NewProvider returns a Provider. istio may be nil if the istio-gateway source
// is never requested.
func NewProvider(kube kubernetes.Interface, istio istioclient.Interface) *Provider {
	return &Provider{kube: kube, istio: istio, built: map[registry.SourceType]builtSource{}}
}

// Endpoints returns the endpoints for kind using its effective config, building
// (or rebuilding on config change) the native external-dns source. parent must
// be the long-lived manager context (informers live for its lifetime).
func (p *Provider) Endpoints(parent context.Context, kind registry.SourceType, cfg *EffectiveConfig) ([]*endpoint.Endpoint, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h := cfg.hash(kind)
	bs, ok := p.built[kind]
	if !ok || bs.hash != h {
		if ok {
			bs.cancel() // stop the previous informer before rebuilding
		}
		ec, err := cfg.toConfig(kind)
		if err != nil {
			return nil, err
		}
		srcCtx, cancel := context.WithCancel(parent)
		src, err := p.build(srcCtx, kind, ec)
		if err != nil {
			cancel()
			delete(p.built, kind)
			return nil, fmt.Errorf("build %s source: %w", kind, err)
		}
		bs = builtSource{src: src, hash: h, cancel: cancel}
		p.built[kind] = bs
	}
	return bs.src.Endpoints(parent)
}

// Forget cancels and drops a kind's source (e.g. when it becomes disabled on
// every DNS CR), stopping its informer.
func (p *Provider) Forget(kind registry.SourceType) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if bs, ok := p.built[kind]; ok {
		bs.cancel()
		delete(p.built, kind)
	}
}

func (p *Provider) build(ctx context.Context, kind registry.SourceType, cfg *externaldnssource.Config) (externaldnssource.Source, error) {
	switch kind {
	case KindService:
		return externaldnssource.NewServiceSource(ctx, p.kube, cfg)
	case KindIngress:
		return externaldnssource.NewIngressSource(ctx, p.kube, cfg)
	case KindIstioGateway:
		if p.istio == nil {
			return nil, fmt.Errorf("istio client not configured")
		}
		return externaldnssource.NewIstioGatewaySource(ctx, p.kube, p.istio, cfg)
	default:
		return nil, fmt.Errorf("externaldns: unsupported kind %q", kind)
	}
}
