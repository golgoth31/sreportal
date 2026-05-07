/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import "context"

// Client is the port the ImageRegistry controller uses to query container
// registries. Implementations live in `internal/registry/` (adapter layer).
//
// ListTags must return the raw tag list as the registry exposes it (no
// filtering, no sort) — selection logic lives in PickLatestSemver.
//
// Implementations are responsible for surfacing rate-limit signals as
// distinguishable errors (see ErrRateLimited).
type Client interface {
	ListTags(ctx context.Context, host, repository string) ([]string, error)
}
