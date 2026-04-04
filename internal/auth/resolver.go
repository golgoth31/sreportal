package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
)

// PortalResolver resolves per-portal authentication for Connect write RPCs using
// Portal projections, optional inheritance from the main portal, and Secret-backed API keys.
type PortalResolver struct {
	client          client.Client
	portalReader    domainportal.PortalReader
	portalNamespace string

	mu    sync.RWMutex
	cache map[string]*cachedAuth
}

type cachedAuth struct {
	chain      *Chain
	jwtClosers []*JWTAuthenticator
}

// NewPortalResolver builds a resolver. Call Close() on shutdown to release JWKS goroutines.
func NewPortalResolver(c client.Client, r domainportal.PortalReader, portalNamespace string) *PortalResolver {
	return &PortalResolver{
		client:          c,
		portalReader:    r,
		portalNamespace: portalNamespace,
		cache:           make(map[string]*cachedAuth),
	}
}

// Close stops background JWKS refresh goroutines held in the auth cache.
func (r *PortalResolver) Close() {
	r.Invalidate()
}

// Invalidate drops cached chains (e.g. after Portal or Secret changes).
func (r *PortalResolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cache {
		for _, j := range c.jwtClosers {
			j.Close()
		}
	}
	r.cache = make(map[string]*cachedAuth)
}

func stableAuthKey(v *domainportal.PortalAuthView) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *PortalResolver) chainFor(ctx context.Context, auth *domainportal.PortalAuthView) (*Chain, error) {
	if auth == nil || !auth.Enabled() {
		return nil, nil
	}
	key, err := stableAuthKey(auth)
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	if c, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return c.chain, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cache[key]; ok {
		return c.chain, nil
	}
	chain, closers, err := r.buildChain(ctx, auth)
	if err != nil {
		return nil, err
	}
	r.cache[key] = &cachedAuth{chain: chain, jwtClosers: closers}
	return chain, nil
}

func (r *PortalResolver) buildChain(ctx context.Context, auth *domainportal.PortalAuthView) (*Chain, []*JWTAuthenticator, error) {
	var auths []Authenticator
	var jwts []*JWTAuthenticator

	if auth.APIKey != nil && auth.APIKey.Enabled {
		var sec corev1.Secret
		err := r.client.Get(ctx, types.NamespacedName{Namespace: r.portalNamespace, Name: auth.APIKey.SecretName}, &sec)
		if err != nil {
			return nil, nil, fmt.Errorf("read API key secret %q: %w", auth.APIKey.SecretName, err)
		}
		raw := sec.Data[auth.APIKey.SecretKey]
		if len(raw) == 0 {
			return nil, nil, fmt.Errorf("secret %q missing data key %q", auth.APIKey.SecretName, auth.APIKey.SecretKey)
		}
		auths = append(auths, NewAPIKeyAuthenticator(auth.APIKey.HeaderName, string(bytes.TrimSpace(raw))))
	}

	if auth.JWT != nil && auth.JWT.Enabled {
		jc := jwtAuthViewToConfig(auth.JWT)
		j, err := NewJWTAuthenticator(ctx, jc)
		if err != nil {
			return nil, nil, fmt.Errorf("jwt authenticator: %w", err)
		}
		jwts = append(jwts, j)
		auths = append(auths, j)
	}

	if len(auths) == 0 {
		return nil, nil, nil
	}
	return NewChain(auths...), jwts, nil
}

// Authenticate enforces portal-scoped auth for protected write procedures.
func (r *PortalResolver) Authenticate(ctx context.Context, proc string, msg any, headers http.Header) error {
	if !WriteProcedures[proc] {
		return nil
	}
	feat, ok := authFeatureForProcedure(proc)
	if !ok {
		return fmt.Errorf("missing auth feature mapping for procedure %s", proc)
	}
	portalRef, err := r.extractPortalRef(ctx, proc, msg)
	if err != nil {
		return err
	}
	portalRef = normalizePortalRef(portalRef)

	views, err := r.portalReader.List(ctx, domainportal.PortalFilters{Namespace: r.portalNamespace})
	if err != nil {
		return fmt.Errorf("list portals: %w", err)
	}

	var mainView *domainportal.PortalView
	var targetView *domainportal.PortalView
	for i := range views {
		v := &views[i]
		if v.Main {
			mainView = v
		}
		if v.Name == portalRef {
			targetView = v
		}
	}
	if targetView == nil {
		targetView = mainView
	}

	eff := effectiveAuth(mainView, targetView, feat)
	chain, err := r.chainFor(ctx, eff)
	if err != nil {
		return err
	}
	if chain == nil {
		return nil
	}
	return chain.Authenticate(ctx, headers)
}
