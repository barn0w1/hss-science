package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// newGatewayMux returns a ServeMux configured for JSON serialization and header mapping.
func newGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		// FIX: Use runtime.MIMEWildcard to apply this JSON config to all requests
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		// Allow passing specific HTTP headers to gRPC metadata (e.g., User-Agent, IP)
		runtime.WithIncomingHeaderMatcher(customHeaderMatcher),
		// Allow gRPC metadata to control HTTP headers (e.g., Set-Cookie, Redirects)
		runtime.WithForwardResponseOption(httpResponseModifier),
	)
}

// withCORS wraps the handler with Cross-Origin Resource Sharing configuration.
func withCORS(h http.Handler, isDev bool) http.Handler {
	options := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-HSS-Request", "Cookie"},
		AllowCredentials: true,
		Debug:            isDev,
	}

	if isDev {
		options.AllowedOrigins = []string{"http://localhost:*"}
	} else {
		// Strict origin check for production
		options.AllowedOrigins = []string{"https://*.hss-science.org"}
	}

	return cors.New(options).Handler(h)
}

// customHeaderMatcher allows critical headers to pass through to the gRPC context.
func customHeaderMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case "x-forwarded-for", "x-real-ip":
		return key, true
	case "user-agent", "x-user-agent":
		return key, true
	case "cookie":
		return key, true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}

// httpResponseModifier translates gRPC metadata into HTTP response headers.
// This enables setting cookies and redirects from gRPC services.
func httpResponseModifier(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	// Handle Set-Cookie
	if cookies := md.HeaderMD.Get("set-cookie"); len(cookies) > 0 {
		for _, cookie := range cookies {
			w.Header().Add("Set-Cookie", cookie)
		}
	}

	// Handle redirects via "x-http-code" metadata
	if codes := md.HeaderMD.Get("x-http-code"); len(codes) > 0 {
		if code, err := strconv.Atoi(codes[0]); err == nil {
			w.WriteHeader(code)
		}
	}

	// Handle Location header for redirects
	if locs := md.HeaderMD.Get("location"); len(locs) > 0 {
		w.Header().Set("Location", locs[0])
	}

	return nil
}
