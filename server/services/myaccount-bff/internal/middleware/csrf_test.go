package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRF(t *testing.T) {
	sentinel := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRF(sentinel)

	readOnly := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, method := range readOnly {
		t.Run("allows_"+method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
		})
	}

	mutating := []string{http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodPut}
	for _, method := range mutating {
		t.Run("blocks_"+method+"_without_header", func(t *testing.T) {
			r := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d", w.Code)
			}
		})

		t.Run("allows_"+method+"_with_header", func(t *testing.T) {
			r := httptest.NewRequest(method, "/", nil)
			r.Header.Set("X-Requested-With", "XMLHttpRequest")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
		})
	}
}
