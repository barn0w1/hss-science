package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/domerr"
)

var _ oidc.ClientRepository = (*ClientRepository)(nil)

type ClientRepository struct {
	db *sqlx.DB
}

func NewClientRepository(db *sqlx.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) GetByID(ctx context.Context, clientID string) (*oidc.Client, error) {
	row := r.db.QueryRowxContext(ctx,
		`SELECT id, secret_hash, redirect_uris, post_logout_redirect_uris,
		        application_type, auth_method, response_types, grant_types,
		        access_token_type, allowed_scopes, id_token_lifetime_seconds, clock_skew_seconds,
		        id_token_userinfo_assertion, created_at, updated_at
		 FROM clients WHERE id = $1`, clientID,
	)

	var c oidc.Client
	var redirectURIs, postLogoutURIs, responseTypes, grantTypes, allowedScopes pq.StringArray
	err := row.Scan(
		&c.ID, &c.SecretHash, &redirectURIs, &postLogoutURIs,
		&c.ApplicationType, &c.AuthMethod, &responseTypes, &grantTypes,
		&c.AccessTokenType, &allowedScopes, &c.IDTokenLifetimeSeconds, &c.ClockSkewSeconds,
		&c.IDTokenUserinfoAssertion, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("client %s: %w", clientID, domerr.ErrNotFound)
		}
		return nil, err
	}
	c.RedirectURIs = redirectURIs
	c.PostLogoutRedirectURIs = postLogoutURIs
	c.ResponseTypes = responseTypes
	c.GrantTypes = grantTypes
	c.AllowedScopes = allowedScopes
	return &c, nil
}
