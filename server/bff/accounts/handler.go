package accounts

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"

	oapi "github.com/barn0w1/hss-science/server/bff/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/bff/internal/session"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// contextKey is a private type for context keys in this package.
type contextKey int

const requestKey contextKey = iota

// RequestInjector is a strict middleware that stores the *http.Request in the context
// so that strict server handlers can access it for session cookie reading.
func RequestInjector(f oapi.StrictHandlerFunc, _ string) oapi.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
		ctx = context.WithValue(ctx, requestKey, r)
		return f(ctx, w, r, request)
	}
}

// requestFromContext retrieves the *http.Request from the context.
func requestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey).(*http.Request)
	return r
}

// Handler implements the oapi-codegen StrictServerInterface for the Accounts BFF.
type Handler struct {
	grpc     accountsv1.AccountsServiceClient
	session  *session.Manager
	config   *Config
	provider string
}

// NewHandler creates a new BFF handler.
func NewHandler(grpc accountsv1.AccountsServiceClient, sess *session.Manager, cfg *Config) *Handler {
	return &Handler{
		grpc:     grpc,
		session:  sess,
		config:   cfg,
		provider: cfg.Provider,
	}
}

// StartOAuthFlow handles GET /authorize.
func (h *Handler) StartOAuthFlow(ctx context.Context, request oapi.StartOAuthFlowRequestObject) (oapi.StartOAuthFlowResponseObject, error) {
	audience := request.Params.Audience
	redirectURI := request.Params.RedirectUri
	clientState := ""
	if request.Params.State != nil {
		clientState = *request.Params.State
	}

	// Validate audience.
	if !h.config.ValidateAudience(audience) {
		return oapi.StartOAuthFlow400JSONResponse{
			Code:    "invalid_audience",
			Message: fmt.Sprintf("unknown audience: %s", audience),
		}, nil
	}

	// Validate redirect_uri.
	if !h.config.ValidateRedirectURI(audience, redirectURI) {
		return oapi.StartOAuthFlow400JSONResponse{
			Code:    "invalid_redirect_uri",
			Message: "redirect_uri is not allowed for this audience",
		}, nil
	}

	// Check for active session — if we have one, issue an auth code directly (true SSO).
	if r := requestFromContext(ctx); r != nil {
		sessionData, _ := h.session.Decode(r)
		if sessionData != nil && h.session.IsValid(sessionData) {
			resp, err := h.grpc.IssueAuthCode(ctx, &accountsv1.IssueAuthCodeRequest{
				UserId: sessionData.UserID,
			})
			if err == nil {
				location := buildRedirectURL(redirectURI, resp.AuthCode, clientState)
				return oapi.StartOAuthFlow302Response{
					Headers: oapi.StartOAuthFlow302ResponseHeaders{
						Location: location,
					},
				}, nil
			}
			// Fall through to provider redirect if IssueAuthCode fails
			// (user may have been deleted).
		}
	}

	// No session or session invalid — redirect to OAuth provider.
	return h.redirectToProvider(ctx, redirectURI, clientState)
}

// OAuthCallback handles GET /oauth/callback.
func (h *Handler) OAuthCallback(ctx context.Context, request oapi.OAuthCallbackRequestObject) (oapi.OAuthCallbackResponseObject, error) {
	resp, err := h.grpc.HandleProviderCallback(ctx, &accountsv1.HandleProviderCallbackRequest{
		Provider: h.provider,
		Code:     request.Params.Code,
		State:    request.Params.State,
	})
	if err != nil {
		return oapi.OAuthCallback401JSONResponse{
			Code:    "authentication_failed",
			Message: "authentication failed or state mismatch",
		}, nil
	}

	// Create session cookie.
	cookieStr, err := h.session.Encode(&session.Data{
		UserID:   resp.User.Id,
		IssuedAt: time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("encode session: %w", err)
	}

	location := buildRedirectURL(resp.RedirectUri, resp.AuthCode, resp.ClientState)
	return oapi.OAuthCallback302Response{
		Headers: oapi.OAuthCallback302ResponseHeaders{
			Location:  location,
			SetCookie: cookieStr,
		},
	}, nil
}

// GetCurrentUser handles GET /me.
func (h *Handler) GetCurrentUser(ctx context.Context, _ oapi.GetCurrentUserRequestObject) (oapi.GetCurrentUserResponseObject, error) {
	r := requestFromContext(ctx)
	if r == nil {
		return oapi.GetCurrentUser401JSONResponse{
			Code:    "unauthenticated",
			Message: "no active session",
		}, nil
	}

	sessionData, _ := h.session.Decode(r)
	if sessionData == nil || !h.session.IsValid(sessionData) {
		return oapi.GetCurrentUser401JSONResponse{
			Code:    "unauthenticated",
			Message: "no active session",
		}, nil
	}

	resp, err := h.grpc.GetUser(ctx, &accountsv1.GetUserRequest{
		UserId: sessionData.UserID,
	})
	if err != nil {
		return oapi.GetCurrentUser401JSONResponse{
			Code:    "user_not_found",
			Message: "session references a non-existent user",
		}, nil
	}

	userID, _ := uuid.Parse(resp.User.Id)
	return oapi.GetCurrentUser200JSONResponse{
		Id:          openapi_types.UUID(userID),
		DisplayName: resp.User.DisplayName,
		AvatarUrl:   resp.User.AvatarUrl,
	}, nil
}

func (h *Handler) redirectToProvider(ctx context.Context, redirectURI, clientState string) (oapi.StartOAuthFlowResponseObject, error) {
	resp, err := h.grpc.GetAuthURL(ctx, &accountsv1.GetAuthURLRequest{
		Provider:    h.provider,
		RedirectUri: redirectURI,
		ClientState: clientState,
	})
	if err != nil {
		return oapi.StartOAuthFlow400JSONResponse{
			Code:    "auth_url_error",
			Message: "failed to generate authorization URL",
		}, nil
	}

	return oapi.StartOAuthFlow302Response{
		Headers: oapi.StartOAuthFlow302ResponseHeaders{
			Location: resp.AuthUrl,
		},
	}, nil
}

func buildRedirectURL(baseURL, authCode, clientState string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	q := u.Query()
	q.Set("code", authCode)
	if clientState != "" {
		q.Set("state", clientState)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Compile-time check: Handler implements StrictServerInterface.
var _ oapi.StrictServerInterface = (*Handler)(nil)
