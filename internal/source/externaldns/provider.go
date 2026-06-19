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
	"errors"
	"fmt"
	"sync"
	"time"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/external-dns/endpoint"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// ErrSourceNotReady is returned while a kind's source is still being built (its
// informer cache has not synced yet). It is NOT a failure: the caller must
// preserve the previous good state and retry on the next cycle, never count it
// as an error or wipe the store.
var ErrSourceNotReady = errors.New("externaldns: source not ready (informer cache syncing)")

// defaultBuildWait bounds how long Endpoints blocks waiting for a freshly
// started source to finish its initial cache sync. A healthy cluster syncs in
// well under this, so data lands on the very first cycle; an unsyncable source
// (missing RBAC, absent CRD) returns ErrSourceNotReady after the cap instead of
// blocking the SourceReconciler forever, and keeps building in the background.
const defaultBuildWait = 60 * time.Second

type builtSource struct {
	src      externaldnssource.Source
	hash     string
	cancel   context.CancelFunc
	ready    bool
	done     chan struct{} // closed by runBuild when the build attempt finishes
	buildErr error         // set before done is closed
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
// regardless of the number of DNS CRs (Option B).
//
// Building is asynchronous and per-kind isolated: the native constructor blocks
// on WaitForCacheSync, so it runs in a goroutine on the long-lived context while
// Endpoints waits at most defaultBuildWait. A kind that cannot sync (missing
// RBAC, absent CRD) therefore never hangs the single-goroutine SourceReconciler
// nor the other kinds — it surfaces ErrSourceNotReady and is retried.
type Provider struct {
	kube      kubernetes.Interface
	istio     istioclient.Interface
	buildWait time.Duration

	mu    sync.Mutex
	built map[registry.SourceType]*builtSource
}

// NewProvider returns a Provider. istio may be nil if the istio-gateway source
// is never requested.
func NewProvider(kube kubernetes.Interface, istio istioclient.Interface) *Provider {
	return &Provider{
		kube:      kube,
		istio:     istio,
		buildWait: defaultBuildWait,
		built:     map[registry.SourceType]*builtSource{},
	}
}

// Endpoints returns the endpoints for kind using its effective config. parent
// must be the long-lived manager context (informers live for its lifetime).
//
// On the first call for a (kind, config) it starts the source build in the
// background and waits up to buildWait for it; if the build is still running it
// returns ErrSourceNotReady (caller preserves state, retries next cycle). A
// config change cancels the old source and rebuilds.
func (p *Provider) Endpoints(parent context.Context, kind registry.SourceType, cfg *EffectiveConfig) ([]*endpoint.Endpoint, error) {
	h := cfg.hash(kind)

	p.mu.Lock()
	bs := p.built[kind]
	if bs != nil && bs.hash == h {
		if bs.ready {
			src := bs.src
			p.mu.Unlock()
			return src.Endpoints(parent)
		}
		// A build for the desired config is already in flight (started by an
		// earlier cycle): poll without blocking this cycle.
		select {
		case <-bs.done:
			return p.finalizeLocked(kind, bs, parent)
		default:
			p.mu.Unlock()
			return nil, ErrSourceNotReady
		}
	}

	// Not built yet, or the config changed: cancel any stale source and start a
	// fresh background build.
	if bs != nil {
		bs.cancel()
	}
	srcCtx, cancel := context.WithCancel(parent)
	nb := &builtSource{hash: h, cancel: cancel, done: make(chan struct{})}
	p.built[kind] = nb
	p.mu.Unlock()

	go p.runBuild(srcCtx, kind, cfg, nb)

	// Bounded wait so a healthy cluster still delivers on this very cycle.
	select {
	case <-nb.done:
		p.mu.Lock()
		return p.finalizeLocked(kind, nb, parent)
	case <-time.After(p.buildWait):
		return nil, ErrSourceNotReady
	}
}

// finalizeLocked consumes a completed build. Called with p.mu held; always
// releases it.
func (p *Provider) finalizeLocked(kind registry.SourceType, bs *builtSource, parent context.Context) ([]*endpoint.Endpoint, error) {
	if bs.buildErr != nil {
		// Drop so the next cycle retries (e.g. once the CRD/RBAC appears), but
		// only if this entry is still the current one.
		if p.built[kind] == bs {
			delete(p.built, kind)
		}
		err := bs.buildErr
		p.mu.Unlock()
		return nil, fmt.Errorf("build %s source: %w", kind, err)
	}
	bs.ready = true
	src := bs.src
	p.mu.Unlock()
	return src.Endpoints(parent)
}

// runBuild constructs the source (blocking on cache sync) on the long-lived
// ctx, records the result on bs, and closes bs.done.
func (p *Provider) runBuild(ctx context.Context, kind registry.SourceType, cfg *EffectiveConfig, bs *builtSource) {
	logger := log.FromContext(ctx).WithName("externaldns.provider")
	logger.Info("building external-dns source (waiting for informer cache sync)", "kind", kind)

	var src externaldnssource.Source
	ec, err := cfg.toConfig(kind)
	if err == nil {
		src, err = p.build(ctx, kind, ec)
	}

	p.mu.Lock()
	if p.built[kind] == bs {
		bs.src = src
		bs.buildErr = err
	} else if err == nil {
		// Superseded (config changed) or forgotten while building: tear down the
		// informer we just created so it doesn't leak.
		bs.cancel()
	}
	p.mu.Unlock()
	close(bs.done)

	if err != nil {
		logger.Error(err, "external-dns source build failed; will retry next cycle", "kind", kind)
		return
	}
	logger.Info("external-dns source ready", "kind", kind)
}

// Forget cancels and drops a kind's source (e.g. when it becomes disabled on
// every DNS CR), stopping its informer (or an in-flight build).
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
