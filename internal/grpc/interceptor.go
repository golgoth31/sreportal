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

package grpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/golgoth31/sreportal/internal/log"
)

// LoggingInterceptor returns a Connect interceptor that logs handler errors
// at WARN level. Connect errors are invisible to HTTP middleware because the
// protocol returns HTTP 200 even when the handler returns a coded error.
func LoggingInterceptor() connect.UnaryInterceptorFunc {
	logger := log.Default().WithName("connect")

	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err != nil {
				code := connect.CodeOf(err)
				logger.Warn("request error",
					"procedure", req.Spec().Procedure,
					"code", code.String(),
					"error", err.Error(),
				)
			}
			return resp, err
		}
	}
}
