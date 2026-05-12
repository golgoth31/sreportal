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

// Package chain contains Chain-of-Responsibility handlers for the DNSRecord controller.
package chain

// ChainData holds shared state between handlers during a DNSRecord reconciliation.
type ChainData struct {
	// ResourceKey is the namespaced key used to address the read store entry.
	ResourceKey string
}
