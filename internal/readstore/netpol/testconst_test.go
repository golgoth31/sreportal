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

package netpol

// Shared string constants for tests in this package — extracted to satisfy
// goconst and keep test data consistent.
const (
	tNs1             = "ns1"
	tNs2             = "ns2"
	tEdgeInternal    = "internal"
	tNodeTypeService = "service"
	tNodeSvcNs1A     = "svc:ns1:a"
	tNodeSvcNs1B     = "svc:ns1:b"
	tNodeSvcNs2B     = "svc:ns2:b"
	tNodeSvcNs1API   = "svc:ns1:api"
	tNodeSvcNs1Other = "svc:ns1:other"
)
