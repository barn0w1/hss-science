package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/oidcrp"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/session"
)

type contextKey int

const ctxKeySession contextKey = iota

var ErrRefreshFailed = errors.New("refresh failed")

func SessionFromContext(ctx context.Context) *session.Session {
	s, _ := ctx.Value(ctxKeySession).(*session.Session)
	return s
}

func clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__Host-sid",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeErrorJSON(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`{"error":"` + code + `","message":"` + message + `"}`))
}

func Auth(store *session.Store, oidcRP *oidcrp.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("__Host-sid")
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
				return
			}
			sid := cookie.Value

			sess, err := store.Load(r.Context(), sid)
			if errors.Is(err, session.ErrNotFound) {
				clearCookie(w)
				writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "session expired")
				return
			}
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "internal", "session store error")
				return
			}

			if time.Until(sess.TokenExpiry) < 60*time.Second {
				sess, err = refreshTokens(r.Context(), store, oidcRP, sid, sess)
				if err != nil {
					if errors.Is(err, ErrRefreshFailed) {
						_ = store.Delete(r.Context(), sid)
						clearCookie(w)
						writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "session expired")
						return
					}
					writeErrorJSON(w, http.StatusInternalServerError, "internal", "token refresh error")
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxKeySession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func refreshTokens(ctx context.Context, store *session.Store, rp *oidcrp.Client, sid string, sess *session.Session) (*session.Session, error) {
	acquired, err := store.AcquireRefreshLock(ctx, sid)
	if err != nil {
		return nil, err
	}
	if !acquired {
		time.Sleep(500 * time.Millisecond)
		reloaded, err := store.Load(ctx, sid)
		if errors.Is(err, session.ErrNotFound) {
			return nil, ErrRefreshFailed
		}
		return reloaded, err
	}
	defer func() { _ = store.ReleaseRefreshLock(ctx, sid) }()

	if time.Until(sess.TokenExpiry) >= 60*time.Second {
		return sess, nil
	}

	newToken, _, err := rp.RefreshToken(ctx, sess.RefreshToken)
	if err != nil {
		return nil, ErrRefreshFailed
	}

	sess.AccessToken = newToken.AccessToken
	if newToken.RefreshToken != "" {
		sess.RefreshToken = newToken.RefreshToken
	}
	sess.TokenExpiry = newToken.Expiry
	if rawIDToken, ok := newToken.Extra("id_token").(string); ok && rawIDToken != "" {
		sess.IDToken = rawIDToken
	}
	if newDSID := oidcrp.ExtractDSID(newToken.AccessToken); newDSID != "" {
		sess.DeviceSessionID = newDSID
	}
	sess.LastActiveAt = time.Now().UTC()

	if err := store.Save(ctx, sid, sess); err != nil {
		return nil, err
	}
	return sess, nil
}
