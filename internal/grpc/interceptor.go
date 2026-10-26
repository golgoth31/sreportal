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

	"connectrpc.com/connect/v2"

	"github.com/golgoth31/sreportal/internal/log"
)

// LoggingInterceptor returns a Connect server interceptor that logs handler errors
// at WARN level. Connect errors are invisible to HTTP middleware because the
// protocol returns HTTP 200 even when the handler returns a coded error.
func LoggingInterceptor() connect.ServerInterceptor {
	logger := log.Default().WithName("connect")

	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			err := next(ctx, spec, stream)
			if err != nil {
				logger.Warn("request error",
					"procedure", spec.Procedure,
					"code", connect.CodeOf(err).String(),
					"error", err.Error(),
				)
			}
			return err
		}
	}
}
