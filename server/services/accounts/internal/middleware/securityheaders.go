package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders adds standard security response headers to every response.
// This configuration is suitable for an Identity Provider (IdP)
// serving HTML login pages.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			h := w.Header()

			// Prevent MIME sniffing
			h.Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			h.Set("X-Frame-Options", "DENY")

			// Control referrer information
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Enforce HTTPS
			h.Set("Strict-Transport-Security",
				"max-age=63072000; includeSubDomains; preload")

			// Content Security Policy (balanced for login pages)
			cspDirectives := []string{
				"default-src 'self'",
				"script-src 'self' https://static.cloudflareinsights.com",
				"style-src 'self'",
				"img-src 'self' data:",
				"connect-src 'self'",
				"form-action 'self'",
				"base-uri 'self'",
				"frame-ancestors 'none'",
			}

			h.Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))

			// Restrict powerful browser features
			h.Set("Permissions-Policy",
				"geolocation=(), camera=(), microphone=(), payment=(), usb=()")

			next.ServeHTTP(w, r)
		})
	}
}
