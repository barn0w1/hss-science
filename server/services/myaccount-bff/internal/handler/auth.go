package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/oidcrp"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/session"
)

type AuthHandler struct {
	store  *session.Store
	oidcRP *oidcrp.Client
}

func NewAuth(store *session.Store, oidcRP *oidcrp.Client) *AuthHandler {
	return &AuthHandler{store: store, oidcRP: oidcRP}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	authURL, state, verifier := h.oidcRP.AuthCodeURL()
	if err := h.store.SaveState(r.Context(), state, verifier); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to save state")
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "code and state are required")
		return
	}

	verifier, err := h.store.LoadAndDeleteState(r.Context(), state)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_state", "invalid or expired state")
		return
	}

	token, idToken, err := h.oidcRP.Exchange(r.Context(), code, verifier)
	if err != nil {
		writeError(w, http.StatusBadRequest, "exchange_failed", "token exchange failed")
		return
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	dsid := oidcrp.ExtractDSID(token.AccessToken)
	now := time.Now().UTC()

	sess := &session.Session{
		UserID:          idToken.Subject,
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		IDToken:         rawIDToken,
		TokenExpiry:     token.Expiry,
		DeviceSessionID: dsid,
		CreatedAt:       now,
		LastActiveAt:    now,
	}

	sid := ulid.Make().String()
	if err := h.store.Save(r.Context(), sid, sess); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to save session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "__Host-sid",
		Value:    sid,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())

	cookie, err := r.Cookie("__Host-sid")
	if err == nil {
		_ = h.store.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "__Host-sid",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	endSessionURL, err := h.oidcRP.EndSessionURL(sess.IDToken, "")
	if err != nil {
		endSessionURL = "/"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"redirect_to": endSessionURL})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("__Host-sid")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"logged_in": false})
		return
	}

	sess, err := h.store.Load(r.Context(), cookie.Value)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"logged_in": false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"logged_in": true,
		"user_id":   sess.UserID,
	})
}
