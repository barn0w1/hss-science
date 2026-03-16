package adapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/oklog/ulid/v2"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ op.Storage = (*StorageAdapter)(nil)

type StorageAdapter struct {
	users       UserClaimsSource
	authReqs    oidcdom.AuthRequestService
	clients     oidcdom.ClientService
	tokens      oidcdom.TokenService
	signing     *SigningKeyWithID
	publicKeys  *PublicKeySet
	accessTTL   time.Duration
	refreshTTL  time.Duration
	healthCheck func(context.Context) error
}

func NewStorageAdapter(
	users UserClaimsSource,
	authReqs oidcdom.AuthRequestService,
	clients oidcdom.ClientService,
	tokens oidcdom.TokenService,
	signing *SigningKeyWithID,
	publicKeys *PublicKeySet,
	accessTTL, refreshTTL time.Duration,
	healthCheck func(context.Context) error,
) *StorageAdapter {
	return &StorageAdapter{
		users:       users,
		authReqs:    authReqs,
		clients:     clients,
		tokens:      tokens,
		signing:     signing,
		publicKeys:  publicKeys,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		healthCheck: healthCheck,
	}
}

func (s *StorageAdapter) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	ar := &oidcdom.AuthRequest{
		ID:                  ulid.Make().String(),
		ClientID:            authReq.ClientID,
		RedirectURI:         authReq.RedirectURI,
		State:               authReq.State,
		Nonce:               authReq.Nonce,
		Scopes:              authReq.Scopes,
		ResponseType:        string(authReq.ResponseType),
		ResponseMode:        string(authReq.ResponseMode),
		CodeChallenge:       authReq.CodeChallenge,
		CodeChallengeMethod: string(authReq.CodeChallengeMethod),
		Prompt:              promptToStrings(authReq.Prompt),
		LoginHint:           authReq.LoginHint,
	}
	if authReq.MaxAge != nil {
		secs := int64(min(*authReq.MaxAge, uint(math.MaxInt64))) //nolint:gosec
		ar.MaxAge = &secs
	}
	if userID != "" {
		ar.UserID = userID
	}

	if client, err := s.clients.GetByID(ctx, ar.ClientID); err == nil {
		if isPublicClient(client.AuthMethod) && ar.CodeChallenge == "" {
			return nil, oidc.ErrInvalidRequest().WithDescription("code_challenge is required for public clients (PKCE S256)")
		}
	}

	if err := s.authReqs.Create(ctx, ar); err != nil {
		return nil, internalErr("create auth request", err)
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	ar, err := s.authReqs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found")
		}
		return nil, internalErr("get auth request by id", err)
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	ar, err := s.authReqs.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found for code")
		}
		return nil, internalErr("get auth request by code", err)
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) SaveAuthCode(ctx context.Context, id, code string) error {
	if err := s.authReqs.SaveCode(ctx, id, code); err != nil {
		return internalErr("save auth code", err)
	}
	return nil
}

func (s *StorageAdapter) DeleteAuthRequest(ctx context.Context, id string) error {
	if err := s.authReqs.Delete(ctx, id); err != nil {
		return internalErr("delete auth request", err)
	}
	return nil
}

func (s *StorageAdapter) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	expiration := time.Now().UTC().Add(s.accessTTL)
	tokenID, err := s.tokens.CreateAccess(ctx,
		clientIDFromRequest(request),
		request.GetSubject(),
		request.GetAudience(),
		request.GetScopes(),
		expiration,
	)
	if err != nil {
		return "", time.Time{}, internalErr("create access token", err)
	}
	return tokenID, expiration, nil
}

func (s *StorageAdapter) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	accessExp := time.Now().UTC().Add(s.accessTTL)
	refreshExp := time.Now().UTC().Add(s.refreshTTL)
	authTime, amr := extractAuthTimeAMR(request)
	deviceSessionID := extractDeviceSessionID(request)

	accessID, refreshToken, err := s.tokens.CreateAccessAndRefresh(ctx,
		clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(),
		accessExp, refreshExp, authTime, amr, currentRefreshToken, deviceSessionID)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return "", "", time.Time{}, op.ErrInvalidRefreshToken
		}
		return "", "", time.Time{}, internalErr("create access and refresh tokens", err)
	}
	return accessID, refreshToken, accessExp, nil
}

func (s *StorageAdapter) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	rt, err := s.tokens.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return nil, op.ErrInvalidRefreshToken
		}
		return nil, internalErr("get refresh token", err)
	}
	return NewRefreshTokenRequest(rt), nil
}

func (s *StorageAdapter) TerminateSession(ctx context.Context, userID, clientID string) error {
	if err := s.tokens.DeleteByUserAndClient(ctx, userID, clientID); err != nil {
		return internalErr("terminate session", err)
	}
	return nil
}

func (s *StorageAdapter) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
	if userID != "" {
		// GetRefreshTokenInfo succeeded — this IS a refresh token
		if err := s.tokens.RevokeRefreshToken(ctx, tokenOrTokenID, clientID); err != nil {
			if errors.Is(err, domerr.ErrNotFound) {
				return oidc.ErrInvalidRequest().WithDescription("token not found")
			}
			return oidc.ErrServerError().WithParent(err)
		}
		return nil
	}
	// GetRefreshTokenInfo failed — this is an access token
	if err := s.tokens.Revoke(ctx, tokenOrTokenID, clientID); err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidRequest().WithDescription("token not found")
		}
		return oidc.ErrServerError().WithParent(err)
	}
	return nil
}

func (s *StorageAdapter) GetRefreshTokenInfo(ctx context.Context, _ string, token string) (string, string, error) {
	userID, tokenID, err := s.tokens.GetRefreshInfo(ctx, token)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return "", "", op.ErrInvalidRefreshToken
		}
		return "", "", internalErr("get refresh token info", err)
	}
	return userID, tokenID, nil
}

func (s *StorageAdapter) SigningKey(_ context.Context) (op.SigningKey, error) {
	return s.signing, nil
}

func (s *StorageAdapter) SignatureAlgorithms(_ context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (s *StorageAdapter) KeySet(_ context.Context) ([]op.Key, error) {
	all := s.publicKeys.All()
	keys := make([]op.Key, len(all))
	for i, k := range all {
		keys[i] = k
	}
	return keys, nil
}

func (s *StorageAdapter) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	c, err := s.clients.GetByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidClient().WithDescription("client not found")
		}
		return nil, internalErr("get client by id", err)
	}
	return NewClientAdapter(c), nil
}

func (s *StorageAdapter) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	err := s.clients.AuthorizeSecret(ctx, clientID, clientSecret)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidClient().WithDescription("client not found")
		}
		if errors.Is(err, domerr.ErrUnauthorized) {
			return oidc.ErrInvalidClient().WithDescription("invalid client secret")
		}
		return internalErr("authorize client secret", err)
	}
	return nil
}

func (s *StorageAdapter) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, _ string, scopes []string) error {
	return s.setUserinfo(ctx, userinfo, userID, scopes)
}

func (s *StorageAdapter) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, request op.IDTokenRequest, scopes []string) error {
	return s.setUserinfo(ctx, userinfo, request.GetSubject(), scopes)
}

func (s *StorageAdapter) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, _, _ string) error {
	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidRequest().WithDescription("token not found")
		}
		return internalErr("get token for userinfo", err)
	}
	return s.setUserinfo(ctx, userinfo, token.Subject, token.Scopes)
}

func (s *StorageAdapter) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, _, _ string) error {
	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			introspection.Active = false
			return nil
		}
		return internalErr("get token for introspection", err)
	}

	introspection.Active = true
	introspection.Subject = token.Subject
	introspection.ClientID = token.ClientID
	introspection.Scope = oidc.SpaceDelimitedArray(token.Scopes)
	introspection.Audience = token.Audience
	introspection.Expiration = oidc.FromTime(token.Expiration)
	introspection.IssuedAt = oidc.FromTime(token.CreatedAt)
	introspection.TokenType = oidc.BearerToken

	user, err := s.users.UserClaims(ctx, token.Subject)
	if err != nil && !errors.Is(err, domerr.ErrNotFound) {
		return internalErr("get user for introspection", err)
	}
	if user != nil {
		s.setIntrospectionUserinfo(introspection, user, token.Scopes)
	}
	return nil
}

func (s *StorageAdapter) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, _ []string) (map[string]any, error) {
	dsid, err := s.tokens.GetLatestDeviceSessionID(ctx, userID, clientID)
	if err != nil {
		slog.Warn("GetPrivateClaimsFromScopes: could not fetch device session id",
			"user_id", userID, "client_id", clientID, "error", err)
		return map[string]any{}, nil
	}
	if dsid == "" {
		return map[string]any{}, nil
	}
	return map[string]any{"dsid": dsid}, nil
}

func (s *StorageAdapter) GetKeyByIDAndClientID(_ context.Context, _, _ string) (*jose.JSONWebKey, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (s *StorageAdapter) ValidateJWTProfileScopes(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (s *StorageAdapter) Health(ctx context.Context) error {
	return s.healthCheck(ctx)
}

func (s *StorageAdapter) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	c, err := s.clients.ClientCredentials(ctx, clientID, clientSecret)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidClient().WithDescription("client not found")
		}
		if errors.Is(err, domerr.ErrUnauthorized) {
			return nil, oidc.ErrInvalidClient().WithDescription("invalid client secret")
		}
		return nil, internalErr("client credentials", err)
	}
	return NewClientAdapter(c), nil
}

func (s *StorageAdapter) ClientCredentialsTokenRequest(_ context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	return &clientCredentialsTokenRequest{
		clientID: clientID,
		scopes:   scopes,
	}, nil
}

func (s *StorageAdapter) setUserinfo(ctx context.Context, userinfo *oidc.UserInfo, userID string, scopes []string) error {
	user, err := s.users.UserClaims(ctx, userID)
	if err != nil {
		if errors.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidRequest().WithDescription("user not found")
		}
		return internalErr("get user for userinfo", err)
	}

	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			userinfo.Subject = user.Subject
		case oidc.ScopeProfile:
			userinfo.Name = user.Name
			userinfo.GivenName = user.GivenName
			userinfo.FamilyName = user.FamilyName
			userinfo.Picture = user.Picture
			userinfo.UpdatedAt = oidc.FromTime(user.UpdatedAt)
		case oidc.ScopeEmail:
			userinfo.Email = user.Email
			userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
		}
	}
	return nil
}

func (s *StorageAdapter) setIntrospectionUserinfo(introspection *oidc.IntrospectionResponse, user *UserClaims, scopes []string) {
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			introspection.Subject = user.Subject
		case oidc.ScopeProfile:
			introspection.Name = user.Name
			introspection.GivenName = user.GivenName
			introspection.FamilyName = user.FamilyName
			introspection.Picture = user.Picture
			introspection.UpdatedAt = oidc.FromTime(user.UpdatedAt)
		case oidc.ScopeEmail:
			introspection.Email = user.Email
			introspection.EmailVerified = oidc.Bool(user.EmailVerified)
		}
	}
}

func clientIDFromRequest(request op.TokenRequest) string {
	if ar, ok := request.(*AuthRequest); ok {
		return ar.GetClientID()
	}
	if rtr, ok := request.(*RefreshTokenRequest); ok {
		return rtr.GetClientID()
	}
	if ccr, ok := request.(*clientCredentialsTokenRequest); ok {
		return ccr.clientID
	}
	slog.Warn("clientIDFromRequest: unhandled request type", "type", fmt.Sprintf("%T", request))
	return ""
}

func extractAuthTimeAMR(request op.TokenRequest) (time.Time, []string) {
	type authTimeGetter interface {
		GetAuthTime() time.Time
	}
	type amrGetter interface {
		GetAMR() []string
	}
	var authTime time.Time
	var amr []string
	if at, ok := request.(authTimeGetter); ok {
		authTime = at.GetAuthTime()
	}
	if ag, ok := request.(amrGetter); ok {
		amr = ag.GetAMR()
	}
	return authTime, amr
}

func extractDeviceSessionID(request op.TokenRequest) string {
	type deviceSessionGetter interface {
		GetDeviceSessionID() string
	}
	if g, ok := request.(deviceSessionGetter); ok {
		return g.GetDeviceSessionID()
	}
	return ""
}

func promptToStrings(prompts []string) []string {
	if len(prompts) == 0 {
		return nil
	}
	return prompts
}

type clientCredentialsTokenRequest struct {
	clientID string
	scopes   []string
}

func (c *clientCredentialsTokenRequest) GetSubject() string    { return c.clientID }
func (c *clientCredentialsTokenRequest) GetAudience() []string { return []string{c.clientID} }
func (c *clientCredentialsTokenRequest) GetScopes() []string   { return c.scopes }

func internalErr(msg string, err error) error {
	slog.Error(msg, "error", err)
	return oidc.ErrServerError().WithDescription("internal error")
}

func isPublicClient(authMethod string) bool {
	return authMethod == "none"
}
