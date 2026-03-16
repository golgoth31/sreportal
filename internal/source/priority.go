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

package source

import (
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// FilterPriorityOrder returns the configured priority list with only enabled
// sources kept. When a source appears in priority but is disabled (or unknown),
// a warning is logged and that source is omitted from the result.
// Pass the same builders slice used by the source factory so behaviour stays consistent.
func FilterPriorityOrder(priority []string, builders []registry.Builder, cfg *config.OperatorConfig) []string {
	if len(priority) == 0 {
		return nil
	}
	if cfg == nil {
		return priority
	}

	logger := log.Default().WithName("source").WithName("priority")
	enabledTypes := make(map[string]bool)
	for _, b := range builders {
		if b.Enabled(cfg) {
			enabledTypes[string(b.Type())] = true
		}
	}

	filtered := make([]string, 0, len(priority))
	for _, name := range priority {
		if !enabledTypes[name] {
			logger.Warn("source in priority list is disabled or unknown and will be ignored",
				"source", name)
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}
