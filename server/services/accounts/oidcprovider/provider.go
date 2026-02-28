package oidcprovider

import (
	"log/slog"

	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/text/language"
)

func NewProvider(issuer string, cryptoKey [32]byte, storage *Storage, logger *slog.Logger) (*op.Provider, error) {
	config := &op.Config{
		CryptoKey:                cryptoKey,
		DefaultLogoutRedirectURI: "/logged-out",
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		AuthMethodPrivateKeyJWT:  false,
		GrantTypeRefreshToken:    true,
		RequestObjectSupported:   false,
		SupportedUILocales:       []language.Tag{language.English, language.Japanese},
	}

	return op.NewProvider(config, storage, op.StaticIssuer(issuer),
		op.WithLogger(logger.WithGroup("oidc")),
	)
}
