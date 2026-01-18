package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func (s *Server) runGateway(ctx context.Context) error {
	if s.gatewayReg == nil {
		return nil
	}

	grpcEndpoint := fmt.Sprintf("localhost:%d", s.cfg.GRPCPort)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// mux の設定を強化
	mux := runtime.NewServeMux(
		// 入力方向: ブラウザの Cookie を gRPC メタデータへ
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			if key == "Cookie" {
				return "cookie", true
			}
			return runtime.DefaultHeaderMatcher(key)
		}),
		// 出力方向: gRPC メタデータの set-cookie を本物の HTTP Set-Cookie ヘッダーへ変換
		runtime.WithForwardResponseOption(func(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
			md, ok := runtime.ServerMetadataFromContext(ctx)
			if !ok {
				return nil
			}
			// "set-cookie" メタデータを取得
			if cookies := md.HeaderMD.Get("set-cookie"); len(cookies) > 0 {
				for _, cookie := range cookies {
					// 標準の Set-Cookie ヘッダーとして書き出す
					w.Header().Add("Set-Cookie", cookie)
				}
			}
			return nil
		}),
	)

	if err := s.gatewayReg(ctx, mux, grpcEndpoint, opts); err != nil {
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.HTTPPort),
		Handler: corsMiddleware(mux, s.cfg.AllowedOrigins),
	}

	go func() {
		<-ctx.Done()
		slog.Info("Shutting down HTTP gateway...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	slog.Info("Starting HTTP gateway", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// CORS Middleware
func corsMiddleware(h http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allow := false

		// Check allowed origins
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allow = true
				break
			}
		}

		if allow {
			// Note: When Allow-Credentials is true, Allow-Origin cannot be "*" in standard browsers.
			// However, since we set it to the request "origin", it works fine.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Cookie")
			w.Header().Set("Access-Control-Allow-Credentials", "true") // Important for Cookies (Refresh Token)
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}
