package handler

import (
	"context"
	"net/http"

	myaccountv1 "github.com/barn0w1/hss-science/server/bff/gen/myaccount/v1"
)

type responseWriterCtxKey struct{}

// ResponseWriterFromContext retrieves the http.ResponseWriter from context.
func ResponseWriterFromContext(ctx context.Context) (http.ResponseWriter, bool) {
	w, ok := ctx.Value(responseWriterCtxKey{}).(http.ResponseWriter)
	return w, ok
}

// InjectHTTPMiddleware is a StrictMiddlewareFunc that injects the
// http.ResponseWriter into the context so strict handler methods
// can set headers (e.g., cookies for Logout).
func InjectHTTPMiddleware(f myaccountv1.StrictHandlerFunc, operationID string) myaccountv1.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
		ctx = context.WithValue(ctx, responseWriterCtxKey{}, w)
		return f(ctx, w, r, request)
	}
}
