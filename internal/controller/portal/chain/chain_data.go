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

package chain

import (
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// ChainData holds typed shared state between Portal reconciliation handlers.
type ChainData struct {
	// Writers (optional, populated by Reconcile before chain execution)
	FQDNWriter      domaindns.FQDNWriter
	ReleaseWriter   domainrelease.ReleaseWriter
	FlowGraphWriter domainnetpol.FlowGraphWriter

	// Runtime state (populated by handlers during the chain)
	RemoteClient *remoteclient.Client
	FetchResult  *remoteclient.FetchResult
}
