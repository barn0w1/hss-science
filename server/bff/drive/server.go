package drive

import (
	"log/slog"
	"net/http"

	driveapi "github.com/barn0w1/hss-science/server/bff/gen/drive/v1"
	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/gorilla/securecookie"
)

// Compile-time check that Server implements the generated ServerInterface.
var _ driveapi.ServerInterface = (*Server)(nil)

// Server implements the generated driveapi.ServerInterface.
type Server struct {
	cfg            *Config
	log            *slog.Logger
	oidc           TokenExchanger
	accountsClient accountsv1.AccountsServiceClient
	secureCookie   *securecookie.SecureCookie
}

// NewServer creates a new Drive BFF server.
func NewServer(
	cfg *Config,
	log *slog.Logger,
	oidc TokenExchanger,
	accountsClient accountsv1.AccountsServiceClient,
) *Server {
	return &Server{
		cfg:            cfg,
		log:            log,
		oidc:           oidc,
		accountsClient: accountsClient,
		secureCookie:   securecookie.New(cfg.CookieHashKey, cfg.CookieBlockKey),
	}
}

// newCookie creates an http.Cookie with the standard security attributes.
func (s *Server) newCookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   s.cfg.CookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}
