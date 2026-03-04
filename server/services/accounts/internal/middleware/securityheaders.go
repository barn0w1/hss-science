package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders adds standard security response headers to every response.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			h := w.Header()

			// --- Standard Security Headers ---
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=(), usb=()")

			// --- Content Security Policy (balanced & production-safe) ---

			// 基本CSP
			csp := strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' https://static.cloudflareinsights.com",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data:",
				"connect-src 'self'",
				"font-src 'self'",
				"frame-ancestors 'none'",
				"base-uri 'self'",
				"form-action 'self'",
			}, "; ")

			h.Set("Content-Security-Policy", csp)

			next.ServeHTTP(w, r)
		})
	}
}
