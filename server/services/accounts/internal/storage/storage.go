package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Compile-time interface checks.
var (
	_ op.Storage                  = &PostgresStorage{}
	_ op.ClientCredentialsStorage = &PostgresStorage{}
)

// PostgresStorage implements op.Storage and op.ClientCredentialsStorage
// with all state persisted in PostgreSQL.
type PostgresStorage struct {
	db         *sqlx.DB
	logger     *slog.Logger
	signingKey *signingKey
}

// NewPostgresStorage creates a new storage backed by PostgreSQL.
func NewPostgresStorage(db *sqlx.DB, sk *signingKey, logger *slog.Logger) *PostgresStorage {
	return &PostgresStorage{
		db:         db,
		logger:     logger,
		signingKey: sk,
	}
}

// ---------------------------------------------------------------------------
// Auth Request Management
// ---------------------------------------------------------------------------

// CreateAuthRequest persists a new authorization request.
func (s *PostgresStorage) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	if len(authReq.Prompt) == 1 && authReq.Prompt[0] == "none" {
		return nil, oidc.ErrLoginRequired()
	}

	req := authRequestToInternal(authReq, userID)
	id := uuid.NewString()

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO auth_requests
			(id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
			 code_challenge, code_challenge_method, prompt, login_hint, max_age_seconds, user_id, done, auth_time)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		 RETURNING id`,
		id,
		req.ClientID,
		req.RedirectURI,
		req.State,
		req.Nonce,
		req.Scopes,
		req.ResponseTypeStr,
		req.ResponseModeStr,
		req.CodeChallengeVal,
		req.CodeChallengeMethod,
		req.Prompt,
		req.LoginHint,
		req.MaxAgeSeconds,
		req.UserID,
		req.IsDone,
		req.AuthTime,
	).Scan(&req.ID)
	if err != nil {
		return nil, fmt.Errorf("insert auth request: %w", err)
	}

	return req, nil
}

// AuthRequestByID retrieves an authorization request by its ID.
func (s *PostgresStorage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	var req AuthRequest
	err := s.db.GetContext(ctx, &req,
		`SELECT id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		        code_challenge, code_challenge_method, prompt, login_hint, max_age_seconds,
		        user_id, done, auth_time, created_at
		 FROM auth_requests WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("request not found")
		}
		return nil, fmt.Errorf("get auth request: %w", err)
	}
	return &req, nil
}

// AuthRequestByCode retrieves authorization request associated with an authorization code.
func (s *PostgresStorage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	var req AuthRequest
	err := s.db.GetContext(ctx, &req,
		`SELECT ar.id, ar.client_id, ar.redirect_uri, ar.state, ar.nonce, ar.scopes,
		        ar.response_type, ar.response_mode, ar.code_challenge, ar.code_challenge_method,
		        ar.prompt, ar.login_hint, ar.max_age_seconds, ar.user_id, ar.done, ar.auth_time,
		        ar.created_at
		 FROM auth_requests ar
		 JOIN auth_codes ac ON ar.id = ac.auth_request_id
		 WHERE ac.code = $1`, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("code invalid or expired")
		}
		return nil, fmt.Errorf("get auth request by code: %w", err)
	}
	return &req, nil
}

// SaveAuthCode maps an authorization code to a request ID.
func (s *PostgresStorage) SaveAuthCode(ctx context.Context, id string, code string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO auth_codes (code, auth_request_id) VALUES ($1, $2)`, code, id)
	if err != nil {
		return fmt.Errorf("save auth code: %w", err)
	}
	return nil
}

// DeleteAuthRequest removes an authorization request and its associated code.
func (s *PostgresStorage) DeleteAuthRequest(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_requests WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete auth request: %w", err)
	}
	return nil
}

// CompleteAuthRequest marks an auth request as done with the authenticated user.
func (s *PostgresStorage) CompleteAuthRequest(ctx context.Context, authRequestID string, userID string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE auth_requests SET user_id = $1, done = true, auth_time = now() WHERE id = $2`,
		userID, authRequestID)
	if err != nil {
		return fmt.Errorf("complete auth request: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("auth request not found: %s", authRequestID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Token Management
// ---------------------------------------------------------------------------

// CreateAccessToken creates an opaque access token.
func (s *PostgresStorage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	var clientID string
	switch req := request.(type) {
	case *AuthRequest:
		clientID = req.ClientID
	case op.TokenExchangeRequest:
		clientID = req.GetClientID()
	}

	tokenID := uuid.NewString()
	expiresAt := time.Now().Add(5 * time.Minute)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO access_tokens (id, client_id, subject, audience, scopes, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tokenID, clientID, request.GetSubject(),
		pq.StringArray(request.GetAudience()),
		pq.StringArray(request.GetScopes()),
		expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create access token: %w", err)
	}
	return tokenID, expiresAt, nil
}

// CreateAccessAndRefreshTokens creates both access and refresh tokens.
// Implements refresh token rotation on refresh requests.
func (s *PostgresStorage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	clientID, authTime, amr := getInfoFromRequest(request)

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	accessTokenID := uuid.NewString()
	accessExpiresAt := time.Now().Add(5 * time.Minute)
	newRefreshTokenValue := uuid.NewString()
	refreshExpiresAt := time.Now().Add(5 * time.Hour)

	// Create access token
	_, err = tx.ExecContext(ctx,
		`INSERT INTO access_tokens (id, client_id, subject, audience, scopes, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		accessTokenID, clientID, request.GetSubject(),
		pq.StringArray(request.GetAudience()),
		pq.StringArray(request.GetScopes()),
		accessExpiresAt)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("insert access token: %w", err)
	}

	if currentRefreshToken == "" {
		// New refresh token (authorization code flow)
		refreshTokenID := uuid.NewString()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO refresh_tokens (id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			refreshTokenID, newRefreshTokenValue, clientID, request.GetSubject(),
			pq.StringArray(request.GetAudience()),
			pq.StringArray(request.GetScopes()),
			authTime, pq.StringArray(amr), accessTokenID, refreshExpiresAt)
		if err != nil {
			return "", "", time.Time{}, fmt.Errorf("insert refresh token: %w", err)
		}
	} else {
		// Refresh token rotation: delete old, create new
		var oldRT RefreshToken
		err = tx.GetContext(ctx, &oldRT,
			`SELECT id, token, client_id, user_id, audience, scopes, auth_time, amr, expires_at
			 FROM refresh_tokens WHERE token = $1`, currentRefreshToken)
		if err != nil {
			return "", "", time.Time{}, fmt.Errorf("lookup refresh token: %w", err)
		}
		if oldRT.ExpiresAt.Before(time.Now()) {
			return "", "", time.Time{}, fmt.Errorf("expired refresh token")
		}

		// Delete old refresh token and its access token
		if _, err = tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, oldRT.ID); err != nil {
			return "", "", time.Time{}, fmt.Errorf("delete old refresh token: %w", err)
		}
		if oldRT.AccessTokenID != nil {
			if _, err = tx.ExecContext(ctx, `DELETE FROM access_tokens WHERE id = $1`, *oldRT.AccessTokenID); err != nil {
				return "", "", time.Time{}, fmt.Errorf("delete old access token: %w", err)
			}
		}

		// Create replacement refresh token
		newRTID := uuid.NewString()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO refresh_tokens (id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			newRTID, newRefreshTokenValue, oldRT.ClientID, oldRT.UserID,
			oldRT.Audience, pq.StringArray(request.GetScopes()),
			oldRT.AuthTime, oldRT.AMR, accessTokenID, refreshExpiresAt)
		if err != nil {
			return "", "", time.Time{}, fmt.Errorf("insert rotated refresh token: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", "", time.Time{}, fmt.Errorf("commit tx: %w", err)
	}

	return accessTokenID, newRefreshTokenValue, accessExpiresAt, nil
}

// TokenRequestByRefreshToken looks up a refresh token for renewal.
func (s *PostgresStorage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	var rt RefreshToken
	err := s.db.GetContext(ctx, &rt,
		`SELECT id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expires_at
		 FROM refresh_tokens WHERE token = $1`, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invalid refresh_token")
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &RefreshTokenRequest{&rt}, nil
}

// TerminateSession removes all tokens for a user/client pair (logout).
func (s *PostgresStorage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM access_tokens WHERE subject = $1 AND client_id = $2`, userID, clientID)
	if err != nil {
		return fmt.Errorf("delete access tokens: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1 AND client_id = $2`, userID, clientID)
	if err != nil {
		return fmt.Errorf("delete refresh tokens: %w", err)
	}
	return nil
}

// GetRefreshTokenInfo looks up a refresh token and returns the token id and user id.
func (s *PostgresStorage) GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (string, string, error) {
	var userID, tokenID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, id FROM refresh_tokens WHERE token = $1 AND client_id = $2`,
		token, clientID).Scan(&userID, &tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", op.ErrInvalidRefreshToken
		}
		return "", "", fmt.Errorf("get refresh token info: %w", err)
	}
	return userID, tokenID, nil
}

// RevokeToken revokes an access or refresh token.
func (s *PostgresStorage) RevokeToken(ctx context.Context, tokenIDOrToken string, userID string, clientID string) *oidc.Error {
	// Try as access token ID first
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM access_tokens WHERE id = $1 AND client_id = $2`, tokenIDOrToken, clientID)
	if err == nil {
		if rows, _ := result.RowsAffected(); rows > 0 {
			return nil
		}
	}

	// Try as refresh token value
	var rtID string
	var accessTokenID *string
	err = s.db.QueryRowContext(ctx,
		`SELECT id, access_token_id FROM refresh_tokens WHERE token = $1 AND client_id = $2`,
		tokenIDOrToken, clientID).Scan(&rtID, &accessTokenID)
	if err != nil {
		// Token not found — per spec, this is not an error
		return nil
	}

	_, _ = s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, rtID)
	if accessTokenID != nil {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM access_tokens WHERE id = $1`, *accessTokenID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Signing Keys
// ---------------------------------------------------------------------------

func (s *PostgresStorage) SigningKey(_ context.Context) (op.SigningKey, error) {
	return s.signingKey, nil
}

func (s *PostgresStorage) SignatureAlgorithms(_ context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{s.signingKey.algorithm}, nil
}

func (s *PostgresStorage) KeySet(_ context.Context) ([]op.Key, error) {
	return LoadAllPublicKeys(s.db)
}

// ---------------------------------------------------------------------------
// Client Management
// ---------------------------------------------------------------------------

func (s *PostgresStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	var client Client
	err := s.db.GetContext(ctx, &client,
		`SELECT id, secret_hash, application_type, auth_method, redirect_uris,
		        post_logout_redirect_uris, response_types, grant_types,
		        access_token_type, id_token_userinfo_assertion, clock_skew_seconds,
		        is_service_account, created_at
		 FROM clients WHERE id = $1`, clientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("client not found")
		}
		return nil, fmt.Errorf("get client: %w", err)
	}
	return &client, nil
}

func (s *PostgresStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	var secretHash *string
	err := s.db.QueryRowContext(ctx,
		`SELECT secret_hash FROM clients WHERE id = $1`, clientID).Scan(&secretHash)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if secretHash == nil {
		return fmt.Errorf("client has no secret configured")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*secretHash), []byte(clientSecret)); err != nil {
		return fmt.Errorf("invalid client secret")
	}
	return nil
}

// ---------------------------------------------------------------------------
// UserInfo
// ---------------------------------------------------------------------------

// SetUserinfoFromScopes implements op.Storage. Use SetUserinfoFromRequest instead.
func (s *PostgresStorage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, clientID string, scopes []string) error {
	return nil
}

// SetUserinfoFromRequest populates userinfo for ID token creation.
func (s *PostgresStorage) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, token op.IDTokenRequest, scopes []string) error {
	return s.populateUserinfo(ctx, userinfo, token.GetSubject(), scopes)
}

// SetUserinfoFromToken populates userinfo for the /userinfo endpoint.
func (s *PostgresStorage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	var t Token
	err := s.db.GetContext(ctx, &t,
		`SELECT id, client_id, subject, audience, scopes, expires_at
		 FROM access_tokens WHERE id = $1`, tokenID)
	if err != nil {
		return fmt.Errorf("token is invalid or has expired")
	}
	if t.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("token is expired")
	}
	return s.populateUserinfo(ctx, userinfo, t.Subject, []string(t.Scopes))
}

// SetIntrospectionFromToken populates an introspection response.
func (s *PostgresStorage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	var t Token
	err := s.db.GetContext(ctx, &t,
		`SELECT id, client_id, subject, audience, scopes, expires_at
		 FROM access_tokens WHERE id = $1`, tokenID)
	if err != nil {
		return fmt.Errorf("token is invalid")
	}

	introspection.Expiration = oidc.FromTime(t.ExpiresAt)
	if t.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("token is expired")
	}

	for _, aud := range t.Audience {
		if aud == clientID {
			userInfo := new(oidc.UserInfo)
			if err := s.populateUserinfo(ctx, userInfo, subject, []string(t.Scopes)); err != nil {
				return err
			}
			introspection.SetUserInfo(userInfo)
			introspection.Scope = oidc.SpaceDelimitedArray(t.Scopes)
			introspection.ClientID = t.ClientID
			return nil
		}
	}
	return fmt.Errorf("token is not valid for this client")
}

func (s *PostgresStorage) populateUserinfo(ctx context.Context, userinfo *oidc.UserInfo, userID string, scopes []string) error {
	var user User
	err := s.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	setUserinfo(&user, userinfo, scopes)
	return nil
}

// ---------------------------------------------------------------------------
// Claims & JWT Profile
// ---------------------------------------------------------------------------

func (s *PostgresStorage) GetPrivateClaimsFromScopes(_ context.Context, userID, clientID string, scopes []string) (map[string]any, error) {
	return nil, nil
}

func (s *PostgresStorage) GetKeyByIDAndClientID(_ context.Context, keyID, clientID string) (*jose.JSONWebKey, error) {
	return nil, fmt.Errorf("JWT profile not supported")
}

func (s *PostgresStorage) ValidateJWTProfileScopes(_ context.Context, userID string, scopes []string) ([]string, error) {
	allowedScopes := make([]string, 0)
	for _, scope := range scopes {
		if scope == oidc.ScopeOpenID {
			allowedScopes = append(allowedScopes, scope)
		}
	}
	return allowedScopes, nil
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *PostgresStorage) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// ---------------------------------------------------------------------------
// Client Credentials (Service Accounts)
// ---------------------------------------------------------------------------

func (s *PostgresStorage) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	var client Client
	err := s.db.GetContext(ctx, &client,
		`SELECT id, secret_hash, application_type, auth_method, redirect_uris,
		        post_logout_redirect_uris, response_types, grant_types,
		        access_token_type, id_token_userinfo_assertion, clock_skew_seconds,
		        is_service_account, created_at
		 FROM clients WHERE id = $1 AND is_service_account = true`, clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client credentials")
	}
	if client.SecretHash == nil {
		return nil, fmt.Errorf("client has no secret configured")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*client.SecretHash), []byte(clientSecret)); err != nil {
		return nil, fmt.Errorf("invalid client credentials")
	}
	return &client, nil
}

func (s *PostgresStorage) ClientCredentialsTokenRequest(_ context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	return &oidc.JWTTokenRequest{
		Subject:  clientID,
		Audience: []string{clientID},
		Scopes:   scopes,
	}, nil
}

// ---------------------------------------------------------------------------
// User Identity Management
// ---------------------------------------------------------------------------

// FindOrCreateUser looks up a user by federated identity. If no match is found,
// a new user is auto-provisioned and the federated identity link is created.
func (s *PostgresStorage) FindOrCreateUser(ctx context.Context, provider, externalSub string, profile UserProfile) (*User, error) {
	// Look up existing federated identity
	var userID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM federated_identities WHERE provider = $1 AND external_sub = $2`,
		provider, externalSub).Scan(&userID)

	if err == nil {
		// User exists — update profile and return
		var user User
		err = s.db.GetContext(ctx, &user,
			`UPDATE users SET email = $1, email_verified = $2, given_name = $3, family_name = $4,
			        picture = $5, locale = $6, updated_at = now()
			 WHERE id = $7
			 RETURNING *`,
			profile.Email, profile.EmailVerified, profile.GivenName, profile.FamilyName,
			profile.Picture, profile.Locale, userID)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		return &user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("lookup federated identity: %w", err)
	}

	// Auto-provision new user
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var user User
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (email, email_verified, given_name, family_name, picture, locale)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, email, email_verified, given_name, family_name, picture, locale, created_at, updated_at`,
		profile.Email, profile.EmailVerified, profile.GivenName, profile.FamilyName,
		profile.Picture, profile.Locale,
	).Scan(&user.ID, &user.Email, &user.EmailVerified, &user.GivenName, &user.FamilyName,
		&user.Picture, &user.Locale, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO federated_identities (user_id, provider, external_sub)
		 VALUES ($1, $2, $3)`,
		user.ID, provider, externalSub)
	if err != nil {
		return nil, fmt.Errorf("insert federated identity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &user, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// getInfoFromRequest extracts client ID, auth time, and AMR from a token request.
func getInfoFromRequest(req op.TokenRequest) (clientID string, authTime time.Time, amr []string) {
	switch r := req.(type) {
	case *AuthRequest:
		return r.ClientID, r.GetAuthTime(), r.GetAMR()
	case *RefreshTokenRequest:
		return r.ClientID, r.AuthTime, []string(r.AMR)
	}
	return "", time.Time{}, nil
}
