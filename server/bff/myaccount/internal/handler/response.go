package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	myaccountv1 "github.com/barn0w1/hss-science/server/bff/gen/myaccount/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// apiError is a sentinel error that carries HTTP status and the generated Error body.
// When a strict handler returns (nil, apiError), HandleStrictError writes it as JSON.
type apiError struct {
	HTTPStatus int
	Body       myaccountv1.Error
}

func (e *apiError) Error() string {
	return e.Body.Code + ": " + e.Body.Message
}

// mapGRPCError converts a gRPC error to an *apiError for the strict server error handler.
func mapGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return &apiError{
			HTTPStatus: http.StatusInternalServerError,
			Body:       myaccountv1.Error{Code: "INTERNAL", Message: "internal error"},
		}
	}

	switch st.Code() {
	case codes.NotFound:
		return &apiError{http.StatusNotFound, myaccountv1.Error{Code: "NOT_FOUND", Message: st.Message()}}
	case codes.Unauthenticated:
		return &apiError{http.StatusUnauthorized, myaccountv1.Error{Code: "UNAUTHENTICATED", Message: st.Message()}}
	case codes.PermissionDenied:
		return &apiError{http.StatusForbidden, myaccountv1.Error{Code: "FORBIDDEN", Message: st.Message()}}
	case codes.InvalidArgument:
		return &apiError{http.StatusBadRequest, myaccountv1.Error{Code: "BAD_REQUEST", Message: st.Message()}}
	case codes.FailedPrecondition:
		return &apiError{http.StatusConflict, myaccountv1.Error{Code: "CONFLICT", Message: st.Message()}}
	case codes.AlreadyExists:
		return &apiError{http.StatusConflict, myaccountv1.Error{Code: "CONFLICT", Message: st.Message()}}
	default:
		return &apiError{http.StatusInternalServerError, myaccountv1.Error{Code: "INTERNAL", Message: "internal error"}}
	}
}

// HandleStrictError is the ResponseErrorHandlerFunc for StrictHTTPServerOptions.
// It converts errors returned by strict handlers into JSON error responses.
func HandleStrictError(w http.ResponseWriter, _ *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ae.HTTPStatus)
		_ = json.NewEncoder(w).Encode(ae.Body)
		return
	}

	// Fallback: unknown error -> 500.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(myaccountv1.Error{Code: "INTERNAL", Message: "internal error"})
}

// writeJSON writes a JSON response with the given status code.
// Used only by the manual auth handlers (Login/Callback).
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response using the generated Error type.
// Used only by the manual auth handlers (Login/Callback).
func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, myaccountv1.Error{Code: code, Message: message})
}
