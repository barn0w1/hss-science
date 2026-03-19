package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"

	pb "github.com/barn0w1/hss-science/server/gen/blob/v1"
	"github.com/barn0w1/hss-science/server/services/blob-service/config"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/app"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/repository/postgres"
	s3storage "github.com/barn0w1/hss-science/server/services/blob-service/internal/storage/s3"
	grpctransport "github.com/barn0w1/hss-science/server/services/blob-service/internal/transport/grpc"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/transport/grpc/interceptor"
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

	oidcProvider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		logger.Error("failed to init OIDC provider", "error", err, "issuer", cfg.OIDCIssuerURL)
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

	storage, err := s3storage.New(cfg.R2Endpoint, cfg.R2Bucket, cfg.R2AccessKeyID, cfg.R2SecretAccessKey)
	if err != nil {
		logger.Error("failed to init R2 client", "error", err)
		os.Exit(1)
	}

	repo := postgres.New(db)
	blobApp := app.New(repo, storage, app.Config{
		PresignPutTTL:           cfg.PresignPutTTL,
		PresignGetMaxTTL:        cfg.PresignGetMaxTTL,
		MultipartThresholdBytes: cfg.MultipartThresholdBytes,
	})

	auth := interceptor.NewAuthInterceptor(oidcProvider, "blob-service")
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(auth.Unary()),
		grpc.ChainStreamInterceptor(auth.Stream()),
	)
	pb.RegisterBlobServiceServer(grpcSrv, grpctransport.NewServer(blobApp))

	listener, err := net.Listen("tcp", cfg.GRPCListenAddr)
	if err != nil {
		logger.Error("failed to listen", "error", err, "addr", cfg.GRPCListenAddr)
		os.Exit(1)
	}

	go func() {
		logger.Info("blob-service gRPC server starting", "addr", cfg.GRPCListenAddr)
		if err := grpcSrv.Serve(listener); err != nil {
			logger.Error("gRPC server exited", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down blob-service")
	grpcSrv.GracefulStop()
	logger.Info("blob-service stopped")
}
