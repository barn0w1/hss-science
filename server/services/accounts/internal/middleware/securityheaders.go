package middleware

import (
	"net/http"
)

// SecurityHeaders adds standard security response headers to every response.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			next.ServeHTTP(w, r)
		})
	}
}
