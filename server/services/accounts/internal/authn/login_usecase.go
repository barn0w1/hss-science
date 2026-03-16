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

func (uc *CompleteFederatedLogin) FindOrCreateUser(
	ctx context.Context, provider string, claims identity.FederatedClaims,
) (*identity.User, error) {
	user, err := uc.identity.FindOrCreateByFederatedLogin(ctx, provider, claims)
	if err != nil {
		return nil, fmt.Errorf("federated login: %w", err)
	}
	return user, nil
}

func (uc *CompleteFederatedLogin) CompleteLogin(
	ctx context.Context, authRequestID, userID, deviceSessionID string,
) error {
	authTime := time.Now().UTC()
	amr := []string{"fed"}
	if err := uc.loginComp.CompleteLogin(ctx, authRequestID, userID, authTime, amr, deviceSessionID); err != nil {
		return fmt.Errorf("complete login: %w", err)
	}
	return nil
}
