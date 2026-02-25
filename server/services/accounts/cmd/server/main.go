package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/text/language"

	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/authn"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/database"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/web"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Setup structured JSON logger
	level := parseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	// Connect to PostgreSQL
	db := database.MustConnect(cfg.DatabaseURL)
	defer db.Close()
	logger.Info("connected to database")

	// Load or generate RSA signing key
	sk, err := storage.LoadOrGenerateSigningKey(db)
	if err != nil {
		log.Fatalf("signing key: %v", err)
	}
	logger.Info("signing key ready")

	// Create storage layer
	store := storage.NewPostgresStorage(db, sk, logger)

	// Create Google authentication provider
	googleProvider, err := authn.NewGoogleProvider(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI)
	if err != nil {
		log.Fatalf("google provider: %v", err)
	}
	logger.Info("google authn provider initialized")

	// Create OpenID Provider
	provider, err := newOP(cfg, store, logger)
	if err != nil {
		log.Fatalf("openid provider: %v", err)
	}

	// Create login handler
	loginHandler := web.NewLogin(
		store,
		googleProvider,
		op.AuthCallbackURL(provider),
		op.NewIssuerInterceptor(provider.IssuerFromRequest),
		logger.With("component", "login"),
	)

	// Build router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	// Post-logout landing page
	router.HandleFunc("/logged-out", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("signed out successfully"))
	})

	// Mount login UI
	router.Mount("/login/", http.StripPrefix("/login", loginHandler.Router()))

	// Mount OpenID Provider (handles /.well-known/openid-configuration, /authorize, /token, /userinfo, /keys, etc.)
	router.Mount("/", provider)

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port, "issuer", cfg.Issuer)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	logger.Info("server stopped")
}

func newOP(cfg *config.Config, store op.Storage, logger *slog.Logger) (*op.Provider, error) {
	opConfig := &op.Config{
		CryptoKey:                cfg.EncryptionKey,
		DefaultLogoutRedirectURI: "/logged-out",
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		AuthMethodPrivateKeyJWT:  false,
		GrantTypeRefreshToken:    true,
		RequestObjectSupported:   false,
		SupportedUILocales:       []language.Tag{language.English},
	}

	var opts []op.Option
	opts = append(opts, op.WithLogger(logger.WithGroup("op")))
	if cfg.DevMode {
		opts = append(opts, op.WithAllowInsecure())
	}

	provider, err := op.NewProvider(opConfig, store, op.StaticIssuer(cfg.Issuer), opts...)
	if err != nil {
		return nil, fmt.Errorf("create OP: %w", err)
	}
	return provider, nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
