package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/accounts/config"
	"github.com/barn0w1/hss-science/server/services/accounts/login"
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

	userRepo := repo.NewUserRepository(db)
	clientRepo := repo.NewClientRepository(db)
	authReqRepo := repo.NewAuthRequestRepository(db)
	tokenRepo := repo.NewTokenRepository(db)

	signingKey := oidcprovider.NewSigningKey(cfg.SigningKey)
	publicKey := oidcprovider.NewPublicKey(cfg.SigningKey)

	storage := oidcprovider.NewStorage(db, userRepo, clientRepo, authReqRepo, tokenRepo, signingKey, publicKey)

	provider, err := oidcprovider.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)
	if err != nil {
		logger.Error("failed to create OIDC provider", "error", err)
		os.Exit(1)
	}

	upstreamProviders, err := login.NewUpstreamProviders(context.Background(), cfg)
	if err != nil {
		logger.Error("failed to initialize upstream providers", "error", err)
		os.Exit(1)
	}

	loginHandler := login.NewHandler(
		upstreamProviders,
		userRepo,
		authReqRepo,
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

	logger.Info("accounts service starting", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil { //nolint:gosec
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
