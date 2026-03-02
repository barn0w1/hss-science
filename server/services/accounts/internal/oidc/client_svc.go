package oidc

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ ClientService = (*clientService)(nil)

type clientService struct {
	repo ClientRepository
}

func NewClientService(repo ClientRepository) ClientService {
	return &clientService{repo: repo}
}

func (s *clientService) GetByID(ctx context.Context, clientID string) (*Client, error) {
	return s.repo.GetByID(ctx, clientID)
}

func (s *clientService) AuthorizeSecret(ctx context.Context, clientID, clientSecret string) error {
	_, err := s.ClientCredentials(ctx, clientID, clientSecret)
	return err
}

func (s *clientService) ClientCredentials(ctx context.Context, clientID, clientSecret string) (*Client, error) {
	c, err := s.repo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(clientSecret)); err != nil {
		return nil, fmt.Errorf("client %s: %w", clientID, domerr.ErrUnauthorized)
	}
	return c, nil
}
