package oidcprovider

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

type ClientReader interface {
	GetByID(ctx context.Context, clientID string) (*model.Client, error)
}

type AuthRequestStore interface {
	Create(ctx context.Context, ar *model.AuthRequest) error
	GetByID(ctx context.Context, id string) (*model.AuthRequest, error)
	GetByCode(ctx context.Context, code string) (*model.AuthRequest, error)
	SaveCode(ctx context.Context, id, code string) error
	CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
	Delete(ctx context.Context, id string) error
}

type TokenStore interface {
	CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (string, error)
	CreateAccessAndRefresh(ctx context.Context, clientID, subject string, audience, scopes []string, accessExpiration, refreshExpiration time.Time, authTime time.Time, amr []string, currentRefreshToken string) (string, string, error)
	GetByID(ctx context.Context, tokenID string) (*model.Token, error)
	GetRefreshToken(ctx context.Context, token string) (*model.RefreshToken, error)
	GetRefreshInfo(ctx context.Context, token string) (string, string, error)
	DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
	Revoke(ctx context.Context, tokenID string) error
	RevokeRefreshToken(ctx context.Context, token string) error
}

type Storage struct {
	db                   *sqlx.DB
	identitySvc          identity.Service
	clientRepo           ClientReader
	authReqRepo          AuthRequestStore
	tokenRepo            TokenStore
	signing              *SigningKeyWithID
	public               *PublicKeyWithID
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration
}

func NewStorage(
	db *sqlx.DB,
	identitySvc identity.Service,
	clientRepo ClientReader,
	authReqRepo AuthRequestStore,
	tokenRepo TokenStore,
	signing *SigningKeyWithID,
	public *PublicKeyWithID,
	accessTokenLifetime time.Duration,
	refreshTokenLifetime time.Duration,
) *Storage {
	return &Storage{
		db:                   db,
		identitySvc:          identitySvc,
		clientRepo:           clientRepo,
		authReqRepo:          authReqRepo,
		tokenRepo:            tokenRepo,
		signing:              signing,
		public:               public,
		accessTokenLifetime:  accessTokenLifetime,
		refreshTokenLifetime: refreshTokenLifetime,
	}
}

// --- AuthStorage ---

func (s *Storage) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	ar := &model.AuthRequest{
		ID:                  uuid.New().String(),
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
		secs := int64(min(*authReq.MaxAge, uint(math.MaxInt64))) //nolint:gosec // clamped to MaxInt64
		ar.MaxAge = &secs
	}
	if userID != "" {
		ar.UserID = userID
	}

	if err := s.authReqRepo.Create(ctx, ar); err != nil {
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	ar, err := s.authReqRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found")
		}
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	ar, err := s.authReqRepo.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found for code")
		}
		return nil, err
	}
	return NewAuthRequest(ar), nil
}

func (s *Storage) SaveAuthCode(ctx context.Context, id, code string) error {
	return s.authReqRepo.SaveCode(ctx, id, code)
}

func (s *Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	return s.authReqRepo.Delete(ctx, id)
}

func (s *Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	expiration := time.Now().UTC().Add(s.accessTokenLifetime)
	tokenID, err := s.tokenRepo.CreateAccess(ctx,
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

func (s *Storage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	accessExp := time.Now().UTC().Add(s.accessTokenLifetime)
	refreshExp := time.Now().UTC().Add(s.refreshTokenLifetime)

	var authTime time.Time
	var amr []string

	type authTimeGetter interface {
		GetAuthTime() time.Time
	}
	type amrGetter interface {
		GetAMR() []string
	}
	if at, ok := request.(authTimeGetter); ok {
		authTime = at.GetAuthTime()
	}
	if ag, ok := request.(amrGetter); ok {
		amr = ag.GetAMR()
	}

	accessID, refreshToken, err := s.tokenRepo.CreateAccessAndRefresh(ctx,
		clientIDFromRequest(request),
		request.GetSubject(),
		request.GetAudience(),
		request.GetScopes(),
		accessExp,
		refreshExp,
		authTime,
		amr,
		currentRefreshToken,
	)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return accessID, refreshToken, accessExp, nil
}

func (s *Storage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	rt, err := s.tokenRepo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, op.ErrInvalidRefreshToken
		}
		return nil, err
	}
	return NewRefreshTokenRequest(rt), nil
}

func (s *Storage) TerminateSession(ctx context.Context, userID, clientID string) error {
	return s.tokenRepo.DeleteByUserAndClient(ctx, userID, clientID)
}

func (s *Storage) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
	if userID != "" {
		if err := s.tokenRepo.Revoke(ctx, tokenOrTokenID); err != nil {
			return oidc.ErrServerError().WithParent(err)
		}
		return nil
	}
	if err := s.tokenRepo.RevokeRefreshToken(ctx, tokenOrTokenID); err != nil {
		return oidc.ErrServerError().WithParent(err)
	}
	return nil
}

func (s *Storage) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (string, string, error) {
	userID, tokenID, err := s.tokenRepo.GetRefreshInfo(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", op.ErrInvalidRefreshToken
		}
		return "", "", err
	}
	return userID, tokenID, nil
}

func (s *Storage) SigningKey(_ context.Context) (op.SigningKey, error) {
	return s.signing, nil
}

func (s *Storage) SignatureAlgorithms(_ context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (s *Storage) KeySet(_ context.Context) ([]op.Key, error) {
	return []op.Key{s.public}, nil
}

// --- OPStorage ---

func (s *Storage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	c, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, oidc.ErrInvalidClient().WithDescription("client not found")
		}
		return nil, err
	}
	return NewClient(c), nil
}

func (s *Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	c, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return oidc.ErrInvalidClient().WithDescription("client not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(clientSecret)); err != nil {
		return oidc.ErrInvalidClient().WithDescription("invalid client secret")
	}
	return nil
}

func (s *Storage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, clientID string, scopes []string) error {
	return s.setUserinfo(ctx, userinfo, userID, scopes)
}

func (s *Storage) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, request op.IDTokenRequest, scopes []string) error {
	return s.setUserinfo(ctx, userinfo, request.GetSubject(), scopes)
}

func (s *Storage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, _ string) error {
	token, err := s.tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return oidc.ErrInvalidRequest().WithDescription("token not found")
		}
		return err
	}
	return s.setUserinfo(ctx, userinfo, token.Subject, token.Scopes)
}

func (s *Storage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	token, err := s.tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	user, err := s.identitySvc.GetUser(ctx, token.Subject)
	if err != nil && !domerr.Is(err, domerr.ErrNotFound) {
		return err
	}
	if user != nil {
		s.setIntrospectionUserinfo(introspection, user, token.Scopes)
	}
	return nil
}

func (s *Storage) GetPrivateClaimsFromScopes(_ context.Context, _, _ string, _ []string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (s *Storage) GetKeyByIDAndClientID(_ context.Context, _, _ string) (*jose.JSONWebKey, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (s *Storage) ValidateJWTProfileScopes(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (s *Storage) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// --- ClientCredentialsStorage ---

func (s *Storage) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	if err := s.AuthorizeClientIDSecret(ctx, clientID, clientSecret); err != nil {
		return nil, err
	}
	return s.GetClientByClientID(ctx, clientID)
}

func (s *Storage) ClientCredentialsTokenRequest(_ context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	return &clientCredentialsTokenRequest{
		clientID: clientID,
		scopes:   scopes,
	}, nil
}

// --- helpers ---

func (s *Storage) setUserinfo(ctx context.Context, userinfo *oidc.UserInfo, userID string, scopes []string) error {
	user, err := s.identitySvc.GetUser(ctx, userID)
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

func (s *Storage) setIntrospectionUserinfo(introspection *oidc.IntrospectionResponse, user *identity.User, scopes []string) {
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
