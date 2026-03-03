package authn

import (
	"context"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
)

type CompleteFederatedLogin struct {
	identity  identity.Service
	loginComp oidcdom.LoginCompleter
}

func NewCompleteFederatedLogin(identitySvc identity.Service, loginComp oidcdom.LoginCompleter) *CompleteFederatedLogin {
	return &CompleteFederatedLogin{
		identity:  identitySvc,
		loginComp: loginComp,
	}
}

func (uc *CompleteFederatedLogin) Execute(ctx context.Context, provider string, claims identity.FederatedClaims, authRequestID string) (string, error) {
	user, err := uc.identity.FindOrCreateByFederatedLogin(ctx, provider, claims)
	if err != nil {
		return "", fmt.Errorf("federated login: %w", err)
	}

	authTime := time.Now().UTC()
	amr := []string{"fed"}
	if err := uc.loginComp.CompleteLogin(ctx, authRequestID, user.ID, authTime, amr); err != nil {
		return "", fmt.Errorf("complete login: %w", err)
	}

	return user.ID, nil
}
