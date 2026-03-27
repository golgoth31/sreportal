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

	"connectrpc.com/connect"
)

// WriteProcedures lists the Connect procedures that require authentication.
var WriteProcedures = map[string]bool{
	"/sreportal.v1.ReleaseService/AddRelease":       true,
	"/sreportal.v1.StatusService/UpsertComponent":    true,
	"/sreportal.v1.StatusService/DeleteComponent":    true,
	"/sreportal.v1.StatusService/UpsertMaintenance":  true,
	"/sreportal.v1.StatusService/DeleteMaintenance":  true,
	"/sreportal.v1.StatusService/UpsertIncident":     true,
	"/sreportal.v1.StatusService/DeleteIncident":     true,
}

// AuthInterceptor returns a Connect unary interceptor that enforces authentication
// on write procedures. Unprotected procedures pass through without auth checks.
func AuthInterceptor(chain *Chain) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !WriteProcedures[req.Spec().Procedure] {
				return next(ctx, req)
			}

			headers := req.Header()
			if err := chain.Authenticate(ctx, headers); err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			return next(ctx, req)
		}
	}
}
