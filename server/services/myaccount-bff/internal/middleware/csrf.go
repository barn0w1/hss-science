package middleware

import (
	"net/http"
)

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodPut:
			if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"csrf","message":"X-Requested-With header required"}`, http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
