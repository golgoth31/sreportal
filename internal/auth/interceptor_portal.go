package auth

import (
	"context"

	"connectrpc.com/connect"
)

// PortalAuthInterceptor returns a Connect unary interceptor that enforces per-portal
// authentication for write procedures (see WriteProcedures).
func PortalAuthInterceptor(r *PortalResolver) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if err := r.Authenticate(ctx, req.Spec().Procedure, req.Any(), req.Header()); err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			return next(ctx, req)
		}
	}
}
