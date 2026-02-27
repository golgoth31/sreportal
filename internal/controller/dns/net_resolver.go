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

package dns

import (
	"context"
	"net"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// Compile-time check that NetResolver implements domaindns.Resolver.
var _ domaindns.Resolver = (*NetResolver)(nil)

// NetResolver adapts net.Resolver to the domain Resolver interface.
type NetResolver struct {
	resolver *net.Resolver
}

// NewNetResolver creates a NetResolver using the default system DNS resolver.
func NewNetResolver() *NetResolver {
	return &NetResolver{resolver: net.DefaultResolver}
}

// LookupHost resolves a hostname to a list of IP addresses.
func (r *NetResolver) LookupHost(ctx context.Context, fqdn string) ([]string, error) {
	return r.resolver.LookupHost(ctx, fqdn)
}

// LookupCNAME resolves a CNAME record for the given hostname.
func (r *NetResolver) LookupCNAME(ctx context.Context, fqdn string) (string, error) {
	return r.resolver.LookupCNAME(ctx, fqdn)
}
