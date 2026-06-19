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

import "github.com/golgoth31/sreportal/internal/domain/forge"

const pendingCap = 50

// ComputeLag filters merge commits out of the CompareResult and caps the list
// at pendingCap (50). Returns the filtered list and whether truncation occurred.
func ComputeLag(cr forge.CompareResult) (pending []forge.Commit, truncated bool) {
	for _, c := range cr.Commits {
		if c.Merge {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) > pendingCap {
		return pending[:pendingCap], true
	}
	return pending, false
}

// StateFor maps an aheadBy count to a state string for a successfully-compared entry.
func StateFor(aheadBy int) string {
	if aheadBy == 0 {
		return "ok"
	}
	return "behind"
}
