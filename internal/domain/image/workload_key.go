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

package image

// WorkloadKey uniquely identifies a scanned Kubernetes workload across the
// cluster. It is used to partition the per-portal image projection so that a
// single workload event can update only its own contribution.
type WorkloadKey struct {
	Kind      string
	Namespace string
	Name      string
}
