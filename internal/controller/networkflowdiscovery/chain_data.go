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

package networkflowdiscovery

import sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"

// ChainData holds typed shared state between NetworkFlowDiscovery reconciliation handlers.
type ChainData struct {
	Nodes []sreportalv1alpha1.FlowNode
	Edges []sreportalv1alpha1.FlowEdge
}
