package interceptor

import (
	"context"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey struct{}

var callerSubKey = contextKey{}

func CallerSub(ctx context.Context) string {
	v, _ := ctx.Value(callerSubKey).(string)
	return v
}

type AuthInterceptor struct {
	verifier *oidc.IDTokenVerifier
}

func NewAuthInterceptor(provider *oidc.Provider, audience string) *AuthInterceptor {
	return &AuthInterceptor{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
	}
}

func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := a.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

func (a *AuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	rawToken, err := extractBearerToken(ctx)
	if err != nil {
		return nil, err
	}

	token, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := token.Claims(&claims); err != nil || claims.Sub == "" {
		return nil, status.Error(codes.Unauthenticated, "missing sub claim")
	}

	return context.WithValue(ctx, callerSubKey, claims.Sub), nil
}

func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(vals[0], prefix) {
		return "", status.Error(codes.Unauthenticated, "authorization must use Bearer scheme")
	}
	return strings.TrimPrefix(vals[0], prefix), nil
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
