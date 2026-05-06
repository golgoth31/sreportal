/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import "errors"

// ErrRateLimited is returned by registry adapters when a 429 response is
// observed. The controller surfaces it through metrics + Status without
// retrying within the same reconcile.
var ErrRateLimited = errors.New("registry rate limited")

// ErrInvalidSpec is returned by the controller's ValidateSpec handler.
var ErrInvalidSpec = errors.New("invalid ImageRegistry spec")
