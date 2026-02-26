package handler

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is the standard error response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// writeNoContent writes a 204 No Content response.
func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, Error{Code: code, Message: message})
}

// writeGRPCError maps a gRPC error to the appropriate HTTP error response.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	switch st.Code() {
	case codes.NotFound:
		writeError(w, http.StatusNotFound, "NOT_FOUND", st.Message())
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", st.Message())
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, "FORBIDDEN", st.Message())
	case codes.InvalidArgument:
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", st.Message())
	case codes.FailedPrecondition:
		writeError(w, http.StatusConflict, "CONFLICT", st.Message())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, "CONFLICT", st.Message())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
