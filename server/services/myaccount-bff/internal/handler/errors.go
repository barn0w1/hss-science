package handler

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcToHTTP(err error) (int, string) {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "internal"
	}
	switch st.Code() {
	case codes.NotFound:
		return http.StatusNotFound, "not_found"
	case codes.InvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case codes.PermissionDenied:
		return http.StatusForbidden, "forbidden"
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "unauthorized"
	case codes.FailedPrecondition:
		return http.StatusConflict, "conflict"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
