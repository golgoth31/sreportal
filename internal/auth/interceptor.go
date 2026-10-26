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

package auth

import (
	"context"
	"net/http"

	"connectrpc.com/connect/v2"
)

// WriteProcedures lists the Connect procedures that require authentication.
var WriteProcedures = map[string]bool{
	"/sreportal.v1.ReleaseService/AddRelease":       true,
	"/sreportal.v1.StatusService/CreateComponent":   true,
	"/sreportal.v1.StatusService/UpdateComponent":   true,
	"/sreportal.v1.StatusService/DeleteComponent":   true,
	"/sreportal.v1.StatusService/CreateMaintenance": true,
	"/sreportal.v1.StatusService/UpdateMaintenance": true,
	"/sreportal.v1.StatusService/DeleteMaintenance": true,
	"/sreportal.v1.StatusService/CreateIncident":    true,
	"/sreportal.v1.StatusService/UpdateIncident":    true,
	"/sreportal.v1.StatusService/DeleteIncident":    true,
}

// AuthInterceptor returns a Connect server interceptor that enforces authentication
// on write procedures. Unprotected procedures pass through without auth checks.
func AuthInterceptor(chain *Chain) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			if !WriteProcedures[spec.Procedure] {
				return next(ctx, spec, stream)
			}

			info, ok := connect.CallInfoForServerContext(ctx)
			if !ok {
				return connect.NewError(connect.CodeInternal, "missing call info")
			}
			if err := chain.Authenticate(ctx, headerToHTTP(info.RequestHeader())); err != nil {
				return connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
			}

			return next(ctx, spec, stream)
		}
	}
}

func headerToHTTP(h *connect.Header) http.Header {
	out := make(http.Header, h.Len())
	for k, vals := range h.All() {
		out[k] = vals
	}
	return out
}
