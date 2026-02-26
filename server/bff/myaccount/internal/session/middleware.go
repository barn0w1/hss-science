package session

import (
	"context"
	"net/http"
)

// CookieName is the session cookie name.
// The __Host- prefix enforces Secure, no Domain, and Path=/.
const CookieName = "__Host-myaccount_session"

type contextKey string

const (
	sessionDataKey contextKey = "session_data"
	sessionIDKey   contextKey = "session_id"
)

// FromContext retrieves the session data from the request context.
func FromContext(ctx context.Context) (*SessionData, bool) {
	sd, ok := ctx.Value(sessionDataKey).(*SessionData)
	return sd, ok
}

// IDFromContext retrieves the session ID from the request context.
func IDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sessionIDKey).(string)
	return id, ok
}

// Middleware returns a chi middleware that validates the session cookie
// and injects session data into the request context.
func Middleware(store *Store, devMode bool) func(http.Handler) http.Handler {
	cookieName := CookieName
	if devMode {
		// __Host- prefix requires Secure which is not available in dev mode.
		cookieName = "myaccount_session"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			sd, err := store.Get(r.Context(), cookie.Value)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), sessionDataKey, sd)
			ctx = context.WithValue(ctx, sessionIDKey, cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"code":"UNAUTHENTICATED","message":"no valid session"}`))
}
