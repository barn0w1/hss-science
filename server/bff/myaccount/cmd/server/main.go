package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/config"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/handler"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load configuration from environment.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Setup structured JSON logger.
	level := parseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	// Connect to Redis.
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("invalid redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to redis")

	// Session store.
	sessionStore := session.NewStore(rdb, cfg.SessionMaxAge)

	// OIDC RP: discover the provider.
	oidcProvider, err := gooidc.NewProvider(ctx, cfg.OIDCIssuer)
	if err != nil {
		logger.Error("oidc provider discovery failed", "error", err)
		os.Exit(1)
	}
	logger.Info("oidc provider discovered", "issuer", cfg.OIDCIssuer)

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURI,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile", "offline_access"},
	}
	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: cfg.OIDCClientID})

	// gRPC client to accounts service.
	grpcClient, err := grpcclient.NewClient(cfg.AccountsGRPCAddr)
	if err != nil {
		logger.Error("grpc client creation failed", "error", err)
		os.Exit(1)
	}
	defer grpcClient.Close()
	logger.Info("grpc client ready", "addr", cfg.AccountsGRPCAddr)

	// Create handlers.
	authHandler := handler.NewAuthHandler(
		oauth2Config, verifier, sessionStore,
		cfg.DevMode, cfg.SPAOrigin,
		logger.With("component", "auth"),
	)
	profileHandler := handler.NewProfileHandler(grpcClient)
	linkedAccountsHandler := handler.NewLinkedAccountsHandler(grpcClient)
	sessionsHandler := handler.NewSessionsHandler(grpcClient)
	accountHandler := handler.NewAccountHandler(grpcClient)

	// Build router.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// CORS configuration for SPA.
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.SPAOrigin},
		AllowedMethods:   []string{"GET", "PATCH", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           3600,
	})
	r.Use(corsHandler.Handler)

	// Health check.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	// Auth routes (no session middleware required).
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// Session-required routes.
	r.Group(func(r chi.Router) {
		r.Use(session.Middleware(sessionStore, cfg.DevMode))

		// Auth routes requiring session.
		r.Post("/auth/logout", authHandler.Logout)
		r.Get("/auth/session", authHandler.Session)

		// API v1 routes.
		r.Get("/api/v1/profile", profileHandler.Get)
		r.Patch("/api/v1/profile", profileHandler.Update)
		r.Get("/api/v1/linked-accounts", linkedAccountsHandler.List)
		r.Delete("/api/v1/linked-accounts/{id}", linkedAccountsHandler.Unlink)
		r.Get("/api/v1/sessions", sessionsHandler.List)
		r.Delete("/api/v1/sessions/{id}", sessionsHandler.Revoke)
		r.Delete("/api/v1/account", accountHandler.Delete)
	})

	// Start HTTP server with graceful shutdown.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
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
