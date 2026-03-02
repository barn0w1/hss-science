// internal/oidc/storage.go
package oidc

import (
	"context"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	"github.com/zitadel/oidc/v3/pkg/op"
)

type UserFinder interface {
	GetByID(ctx context.Context, id string) (*identity.User, error)
}

type Storage struct {
	clients  *ClientRepository
	authReqs *AuthRequestRepository
	tokens   *TokenRepository
	users    UserFinder
}

func NewStorage(c *ClientRepository, a *AuthRequestRepository, t *TokenRepository, u UserFinder) *Storage {
	return &Storage{clients: c, authReqs: a, tokens: t, users: u}
}

// zitadel/oidc が要求するメソッド（ここはライブラリの仕様に合わせるため名前が長くなる）
func (s *Storage) GetClientBySessionAndClientID(ctx context.Context, clientID string) (op.Client, error) {
	return s.clients.GetByID(ctx, clientID) // 小さなRepoの美しいメソッドへ委譲
}
