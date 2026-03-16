package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/accounts"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/session"
)

type SessionsHandler struct {
	accounts     *accounts.Client
	sessionStore *session.Store
}

func NewSessions(ac *accounts.Client, store *session.Store) *SessionsHandler {
	return &SessionsHandler{accounts: ac, sessionStore: store}
}

type sessionResponse struct {
	SessionID  string `json:"session_id"`
	DeviceName string `json:"device_name"`
	IPAddress  string `json:"ip_address"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
	IsCurrent  bool   `json:"is_current"`
}

func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())
	sessions, err := h.accounts.ListActiveSessions(r.Context(), sess.AccessToken)
	if err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}

	result := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		resp := sessionResponse{
			SessionID:  s.GetSessionId(),
			DeviceName: s.GetDeviceName(),
			IPAddress:  s.GetIpAddress(),
			IsCurrent:  s.GetSessionId() == sess.DeviceSessionID,
		}
		if s.GetCreatedAt() != nil {
			resp.CreatedAt = s.GetCreatedAt().AsTime().UTC().Format("2006-01-02T15:04:05Z")
		}
		if s.GetLastUsedAt() != nil {
			resp.LastUsedAt = s.GetLastUsedAt().AsTime().UTC().Format("2006-01-02T15:04:05Z")
		}
		result = append(result, resp)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *SessionsHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "sessionID is required")
		return
	}

	if err := h.accounts.RevokeSession(r.Context(), sess.AccessToken, sessionID); err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}

	if sessionID == sess.DeviceSessionID {
		cookie, err := r.Cookie("__Host-sid")
		if err == nil {
			_ = h.sessionStore.Delete(r.Context(), cookie.Value)
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
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SessionsHandler) RevokeAllOthers(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())

	if sess.DeviceSessionID == "" {
		writeError(w, http.StatusConflict, "conflict", "cannot identify current session")
		return
	}

	if err := h.accounts.RevokeAllOtherSessions(r.Context(), sess.AccessToken, sess.DeviceSessionID); err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
