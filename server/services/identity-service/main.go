package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/identity-service/config"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/authn"
	grpcserver "github.com/barn0w1/hss-science/server/services/identity-service/internal/grpc"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
	identitypg "github.com/barn0w1/hss-science/server/services/identity-service/internal/identity/postgres"
	appmiddleware "github.com/barn0w1/hss-science/server/services/identity-service/internal/middleware"
	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
	oidcadapter "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc/adapter"
	oidcpg "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc/postgres"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/crypto"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	subcommand := "server"
	if len(os.Args) > 1 {
		subcommand = os.Args[1]
	}

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

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSecs) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeSecs) * time.Second)

	tokenRepo := oidcpg.NewTokenRepository(db)
	tokenSvc := oidcdom.NewTokenService(tokenRepo)

	switch subcommand {
	case "cleanup":
		runCleanup(context.Background(), tokenSvc, logger)
		return
	case "server":
		runServer(cfg, db, tokenSvc, logger)
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand — valid: server, cleanup")
		os.Exit(2)
	}
}

func runCleanup(ctx context.Context, tokenSvc oidcdom.TokenService, logger *slog.Logger) {
	cutoff := time.Now().UTC()
	access, refresh, err := tokenSvc.DeleteExpired(ctx, cutoff)
	if err != nil {
		logger.Error("token cleanup failed", "error", err)
		os.Exit(1)
	}
	logger.Info("token cleanup complete", "access_tokens_deleted", access, "refresh_tokens_deleted", refresh)
}

func runServer(cfg *config.Config, db *sqlx.DB, tokenSvc oidcdom.TokenService, logger *slog.Logger) {
	identitySvc := identity.NewService(identitypg.NewUserRepository(db))

	authReqRepo := oidcpg.NewAuthRequestRepository(db)
	clientRepo := oidcpg.NewClientRepository(db)

	authReqSvc := oidcdom.NewAuthRequestService(authReqRepo, time.Duration(cfg.AuthRequestTTLMinutes)*time.Minute)
	clientSvc := oidcdom.NewClientService(clientRepo)

	deviceSessionRepo := oidcpg.NewDeviceSessionRepository(db)
	deviceSessionSvc := oidcdom.NewDeviceSessionService(deviceSessionRepo)

	signingKey := oidcadapter.NewSigningKey(cfg.SigningKeys.Current)
	publicKeys := oidcadapter.NewPublicKeySet(cfg.SigningKeys.Current, cfg.SigningKeys.Previous)

	grpcSrv := grpcserver.NewServer(identitySvc, deviceSessionSvc, publicKeys, cfg.Issuer)
	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen on gRPC port", "error", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}
	go func() {
		logger.Info("gRPC server starting", "port", cfg.GRPCPort)
		if err := grpcSrv.Serve(grpcListener); err != nil {
			logger.Error("gRPC server exited", "error", err)
		}
	}()

	storage := oidcadapter.NewStorageAdapter(
		&userClaimsBridge{svc: identitySvc}, authReqSvc, clientSvc, tokenSvc,
		signingKey, publicKeys,
		time.Duration(cfg.AccessTokenLifetimeMinutes)*time.Minute,
		time.Duration(cfg.RefreshTokenLifetimeDays)*24*time.Hour,
		db.PingContext,
	)

	provider, err := oidcadapter.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)
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
		authReqSvc,
		deviceSessionSvc,
		crypto.NewAESCipher(cfg.CryptoKey),
		op.AuthCallbackURL(provider),
		logger,
	)

	var (
		globalLimiter *appmiddleware.IPRateLimiter
		loginLimiter  *appmiddleware.IPRateLimiter
		tokenLimiter  *appmiddleware.IPRateLimiter
	)
	if cfg.RateLimitEnabled {
		globalLimiter = appmiddleware.NewIPRateLimiter(float64(cfg.RateLimitGlobalRPM)/60.0, cfg.RateLimitGlobalRPM/4)
		loginLimiter = appmiddleware.NewIPRateLimiter(float64(cfg.RateLimitLoginRPM)/60.0, 5)
		tokenLimiter = appmiddleware.NewIPRateLimiter(float64(cfg.RateLimitTokenRPM)/60.0, 10)
	}

	router := chi.NewRouter()
	router.Use(chimiddleware.Recoverer)
	router.Use(appmiddleware.SecurityHeaders())
	if cfg.RateLimitEnabled {
		router.Use(globalLimiter.Middleware())
	}

	interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
	router.Route("/login", func(r chi.Router) {
		if cfg.RateLimitEnabled {
			r.Use(loginLimiter.Middleware())
		}
		r.Use(appmiddleware.SecurityHeaders())
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

	if cfg.RateLimitEnabled {
		router.With(tokenPathLimiter(tokenLimiter)).Mount("/", provider)
	} else {
		router.Mount("/", provider)
	}

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go runAuthRequestCleanup(cleanupCtx, authReqSvc, time.Duration(cfg.AuthRequestTTLMinutes)*time.Minute, logger)
	go runTokenCleanupLoop(cleanupCtx, tokenSvc, time.Hour, logger)
	go runDeviceSessionCleanup(cleanupCtx, deviceSessionSvc, 30*24*time.Hour, logger)

	if cfg.RateLimitEnabled {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-cleanupCtx.Done():
					return
				case <-ticker.C:
					globalLimiter.Cleanup(15 * time.Minute)
					loginLimiter.Cleanup(15 * time.Minute)
					tokenLimiter.Cleanup(15 * time.Minute)
				}
			}
		}()
	}

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

	logger.Info("identity service started", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	cleanupCancel()
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

type userClaimsBridge struct {
	svc identity.Service
}

func (b *userClaimsBridge) UserClaims(ctx context.Context, userID string) (*oidcadapter.UserClaims, error) {
	user, err := b.svc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &oidcadapter.UserClaims{
		Subject:       user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		GivenName:     user.GivenName,
		FamilyName:    user.FamilyName,
		Picture:       user.Picture,
		UpdatedAt:     user.UpdatedAt,
	}, nil
}

func runAuthRequestCleanup(ctx context.Context, svc oidcdom.AuthRequestService, ttl time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-ttl)
			n, err := svc.DeleteExpiredBefore(ctx, cutoff)
			if err != nil {
				logger.Error("auth request cleanup failed", "error", err)
				continue
			}
			if n > 0 {
				logger.Info("cleaned up expired auth requests", "count", n)
			}
		}
	}
}

func runTokenCleanupLoop(ctx context.Context, svc oidcdom.TokenService, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC()
			access, refresh, err := svc.DeleteExpired(ctx, cutoff)
			if err != nil {
				logger.Error("token cleanup failed", "error", err)
				continue
			}
			if access > 0 || refresh > 0 {
				logger.Info("cleaned up expired tokens",
					"access_tokens", access,
					"refresh_tokens", refresh,
				)
			}
		}
	}
}

func runDeviceSessionCleanup(ctx context.Context, svc oidcdom.DeviceSessionService, maxAge time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-maxAge)
			n, err := svc.DeleteRevokedBefore(ctx, cutoff)
			if err != nil {
				logger.Error("device session cleanup failed", "error", err)
				continue
			}
			if n > 0 {
				logger.Info("cleaned up revoked device sessions", "count", n)
			}
		}
	}
}

// tokenPathLimiter applies a rate limiter only to the OIDC token and
// introspection endpoint paths, passing all other paths through unrestricted.
func tokenPathLimiter(limiter *appmiddleware.IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/v2/token", "/token", "/oauth/token",
				"/oauth/v2/introspect", "/introspect":
				limiter.Middleware()(next).ServeHTTP(w, r)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
