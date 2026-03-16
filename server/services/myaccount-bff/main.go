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
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/config"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/accounts"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/handler"
	appmiddleware "github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/oidcrp"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/session"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("failed to parse redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rdbOpts)
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "error", err)
		os.Exit(1)
	}

	sessionStore := session.NewStore(rdb, cfg.SessionIdleTTL, cfg.SessionHardTTL)

	oidcRP, err := oidcrp.New(ctx, cfg.OIDCIssuer, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	if err != nil {
		logger.Error("failed to init OIDC RP", "error", err)
		os.Exit(1)
	}

	accountsClient, err := accounts.New(cfg.AccountsGRPC)
	if err != nil {
		logger.Error("failed to create accounts gRPC client", "error", err)
		os.Exit(1)
	}

	authMW := appmiddleware.Auth(sessionStore, oidcRP)

	authH := handler.NewAuth(sessionStore, oidcRP)
	profileH := handler.NewProfile(accountsClient)
	providersH := handler.NewProviders(accountsClient)
	sessionsH := handler.NewSessions(accountsClient, sessionStore)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Requested-With"},
		AllowCredentials: true,
	}).Handler)
	r.Use(securityHeaders)

	r.Get("/api/v1/auth/login", authH.Login)
	r.Get("/api/v1/auth/callback", authH.Callback)
	r.Get("/api/v1/auth/me", authH.Me)

	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Use(appmiddleware.CSRF)

		r.Post("/api/v1/auth/logout", authH.Logout)
		r.Get("/api/v1/profile", profileH.Get)
		r.Patch("/api/v1/profile", profileH.Update)
		r.Get("/api/v1/providers", providersH.List)
		r.Delete("/api/v1/providers/{identityID}", providersH.Unlink)
		r.Get("/api/v1/sessions", sessionsH.List)
		r.Delete("/api/v1/sessions", sessionsH.RevokeAllOthers)
		r.Delete("/api/v1/sessions/{sessionID}", sessionsH.Revoke)
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
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

	logger.Info("myaccount-bff started", "port", cfg.Port)

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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}
