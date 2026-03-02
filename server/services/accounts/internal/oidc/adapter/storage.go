package adapter

import (
	"context"
	"errors"
	"math"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/oklog/ulid/v2"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ op.Storage = (*StorageAdapter)(nil)

type StorageAdapter struct {
	identity    identity.Service
	authReqs    oidcdom.AuthRequestService
	clients     oidcdom.ClientService
	tokens      oidcdom.TokenService
	signing     *SigningKeyWithID
	public      *PublicKeyWithID
	accessTTL   time.Duration
	refreshTTL  time.Duration
	healthCheck func(context.Context) error
}

func NewStorageAdapter(
	identity identity.Service,
	authReqs oidcdom.AuthRequestService,
	clients oidcdom.ClientService,
	tokens oidcdom.TokenService,
	signing *SigningKeyWithID,
	public *PublicKeyWithID,
	accessTTL, refreshTTL time.Duration,
	healthCheck func(context.Context) error,
) *StorageAdapter {
	return &StorageAdapter{
		identity:    identity,
		authReqs:    authReqs,
		clients:     clients,
		tokens:      tokens,
		signing:     signing,
		public:      public,
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

	if err := s.authReqs.Create(ctx, ar); err != nil {
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	ar, err := s.authReqs.GetByID(ctx, id)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found")
		}
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	ar, err := s.authReqs.GetByCode(ctx, code)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found for code")
		}
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *StorageAdapter) SaveAuthCode(ctx context.Context, id, code string) error {
	return s.authReqs.SaveCode(ctx, id, code)
}

func (s *StorageAdapter) DeleteAuthRequest(ctx context.Context, id string) error {
	return s.authReqs.Delete(ctx, id)
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
		return "", time.Time{}, err
	}
	return tokenID, expiration, nil
}

func (s *StorageAdapter) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	accessExp := time.Now().UTC().Add(s.accessTTL)
	refreshExp := time.Now().UTC().Add(s.refreshTTL)
	authTime, amr := extractAuthTimeAMR(request)

	accessID, refreshToken, err := s.tokens.CreateAccessAndRefresh(ctx,
		clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(),
		accessExp, refreshExp, authTime, amr, currentRefreshToken)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return accessID, refreshToken, accessExp, nil
}

func (s *StorageAdapter) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	rt, err := s.tokens.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return nil, op.ErrInvalidRefreshToken
		}
		return nil, err
	}
	return NewRefreshTokenRequest(rt), nil
}

func (s *StorageAdapter) TerminateSession(ctx context.Context, userID, clientID string) error {
	return s.tokens.DeleteByUserAndClient(ctx, userID, clientID)
}

func (s *StorageAdapter) RevokeToken(ctx context.Context, tokenOrTokenID, userID, _ string) *oidc.Error {
	if userID != "" {
		if err := s.tokens.Revoke(ctx, tokenOrTokenID); err != nil {
			return oidc.ErrServerError().WithParent(err)
		}
		return nil
	}
	if err := s.tokens.RevokeRefreshToken(ctx, tokenOrTokenID); err != nil {
		return oidc.ErrServerError().WithParent(err)
	}
	return nil
}

func (s *StorageAdapter) GetRefreshTokenInfo(ctx context.Context, _ string, token string) (string, string, error) {
	userID, tokenID, err := s.tokens.GetRefreshInfo(ctx, token)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return "", "", op.ErrInvalidRefreshToken
		}
		return "", "", err
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
	return []op.Key{s.public}, nil
}

func (s *StorageAdapter) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	c, err := s.clients.GetByID(ctx, clientID)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidClient().WithDescription("client not found")
		}
		return nil, err
	}
	return NewClientAdapter(c), nil
}

func (s *StorageAdapter) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	err := s.clients.AuthorizeSecret(ctx, clientID, clientSecret)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidClient().WithDescription("client not found")
		}
		if domerr.Is(err, domerr.ErrUnauthorized) {
			return oidc.ErrInvalidClient().WithDescription("invalid client secret")
		}
		return err
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
		if domerr.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidRequest().WithDescription("token not found")
		}
		return err
	}
	return s.setUserinfo(ctx, userinfo, token.Subject, token.Scopes)
}

func (s *StorageAdapter) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, _, _ string) error {
	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			introspection.Active = false
			return nil
		}
		return err
	}

	introspection.Active = true
	introspection.Subject = token.Subject
	introspection.ClientID = token.ClientID
	introspection.Scope = oidc.SpaceDelimitedArray(token.Scopes)
	introspection.Audience = token.Audience
	introspection.Expiration = oidc.FromTime(token.Expiration)
	introspection.IssuedAt = oidc.FromTime(token.CreatedAt)
	introspection.TokenType = oidc.BearerToken

	user, err := s.identity.GetUser(ctx, token.Subject)
	if err != nil && !domerr.Is(err, domerr.ErrNotFound) {
		return err
	}
	if user != nil {
		s.setIntrospectionUserinfo(introspection, user, token.Scopes)
	}
	return nil
}

func (s *StorageAdapter) GetPrivateClaimsFromScopes(_ context.Context, _, _ string, _ []string) (map[string]any, error) {
	return map[string]any{}, nil
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
		if domerr.Is(err, domerr.ErrNotFound) {
			return nil, oidc.ErrInvalidClient().WithDescription("client not found")
		}
		if domerr.Is(err, domerr.ErrUnauthorized) {
			return nil, oidc.ErrInvalidClient().WithDescription("invalid client secret")
		}
		return nil, err
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
	user, err := s.identity.GetUser(ctx, userID)
	if err != nil {
		if domerr.Is(err, domerr.ErrNotFound) {
			return oidc.ErrInvalidRequest().WithDescription("user not found")
		}
		return err
	}

	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			userinfo.Subject = user.ID
		case oidc.ScopeProfile:
			userinfo.Name = user.Name
			userinfo.GivenName = user.GivenName
			userinfo.FamilyName = user.FamilyName
			userinfo.Picture = user.Picture
			userinfo.UpdatedAt = oidc.FromTime(user.CreatedAt)
		case oidc.ScopeEmail:
			userinfo.Email = user.Email
			userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
		}
	}
	return nil
}

func (s *StorageAdapter) setIntrospectionUserinfo(introspection *oidc.IntrospectionResponse, user *identity.User, scopes []string) {
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			introspection.Subject = user.ID
		case oidc.ScopeProfile:
			introspection.Name = user.Name
			introspection.GivenName = user.GivenName
			introspection.FamilyName = user.FamilyName
			introspection.Picture = user.Picture
			introspection.UpdatedAt = oidc.FromTime(user.CreatedAt)
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
