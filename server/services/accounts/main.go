package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/accounts/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/authn"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	identitypg "github.com/barn0w1/hss-science/server/services/accounts/internal/identity/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/oidcprovider"
	"github.com/barn0w1/hss-science/server/services/accounts/repo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	identitySvc := identity.NewService(identitypg.NewUserRepository(db))

	clientRepo := repo.NewClientRepository(db)
	authReqRepo := repo.NewAuthRequestRepository(db)
	tokenRepo := repo.NewTokenRepository(db)

	signingKey := oidcprovider.NewSigningKey(cfg.SigningKey)
	publicKey := oidcprovider.NewPublicKey(cfg.SigningKey)

	storage := oidcprovider.NewStorage(
		db, identitySvc, clientRepo, authReqRepo, tokenRepo,
		signingKey, publicKey,
		time.Duration(cfg.AccessTokenLifetimeMinutes)*time.Minute,
		time.Duration(cfg.RefreshTokenLifetimeDays)*24*time.Hour,
	)

	provider, err := oidcprovider.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)
	if err != nil {
		logger.Error("failed to create OIDC provider", "error", err)
		os.Exit(1)
	}

	upstreamProviders, err := authn.NewProviders(context.Background(), authn.Config{
		IssuerURL:          cfg.Issuer,
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		GitHubClientID:     cfg.GitHubClientID,
		GitHubClientSecret: cfg.GitHubClientSecret,
	})
	if err != nil {
		logger.Error("failed to initialize upstream providers", "error", err)
		os.Exit(1)
	}

	loginHandler := authn.NewHandler(
		upstreamProviders,
		identitySvc,
		&authReqAdapter{repo: authReqRepo},
		cfg.CryptoKey,
		op.AuthCallbackURL(provider),
		logger,
	)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
	router.Route("/login", func(r chi.Router) {
		r.Use(interceptor.Handler)
		r.Get("/", loginHandler.SelectProvider)
		r.Post("/select", loginHandler.FederatedRedirect)
		r.Get("/callback", loginHandler.FederatedCallback)
	})

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	router.Get("/logged-out", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("You have been signed out."))
	})

	router.Mount("/", provider)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("accounts service started", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

type authReqAdapter struct {
	repo *repo.AuthRequestRepository
}

func (a *authReqAdapter) GetByID(ctx context.Context, id string) (authn.AuthRequestInfo, error) {
	ar, err := a.repo.GetByID(ctx, id)
	if err != nil {
		return authn.AuthRequestInfo{}, err
	}
	return authn.AuthRequestInfo{ID: ar.ID}, nil
}

func (a *authReqAdapter) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error {
	return a.repo.CompleteLogin(ctx, id, userID, authTime, amr)
}
