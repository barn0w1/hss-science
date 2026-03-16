package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCToHTTP(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantKey  string
	}{
		{"not_found", status.Error(codes.NotFound, "not found"), http.StatusNotFound, "not_found"},
		{"invalid_argument", status.Error(codes.InvalidArgument, "bad input"), http.StatusBadRequest, "invalid_argument"},
		{"permission_denied", status.Error(codes.PermissionDenied, "denied"), http.StatusForbidden, "forbidden"},
		{"unauthenticated", status.Error(codes.Unauthenticated, "unauth"), http.StatusUnauthorized, "unauthorized"},
		{"failed_precondition", status.Error(codes.FailedPrecondition, "conflict"), http.StatusConflict, "conflict"},
		{"internal", status.Error(codes.Internal, "oops"), http.StatusInternalServerError, "internal"},
		{"unknown_code", status.Error(codes.Unavailable, "unavailable"), http.StatusInternalServerError, "internal"},
		{"non_grpc_error", errors.New("plain error"), http.StatusInternalServerError, "internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, key := grpcToHTTP(tc.err)
			if code != tc.wantCode {
				t.Errorf("expected HTTP %d, got %d", tc.wantCode, code)
			}
			if key != tc.wantKey {
				t.Errorf("expected key %q, got %q", tc.wantKey, key)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "invalid_argument", "field x is required")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content-type, got %s", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "invalid_argument" {
		t.Errorf("expected error=invalid_argument, got %q", body["error"])
	}
	if body["message"] != "field x is required" {
		t.Errorf("expected message, got %q", body["message"])
	}
}
