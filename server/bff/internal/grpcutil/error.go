package grpcutil

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPStatusFromGRPC maps a gRPC status code to an HTTP status code.
func HTTPStatusFromGRPC(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch st.Code() {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// MessageFromGRPC extracts the error message from a gRPC status error.
func MessageFromGRPC(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}
	return st.Message()
}
