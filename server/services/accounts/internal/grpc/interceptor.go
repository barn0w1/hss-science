package grpcserver

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	oidcadapter "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc/adapter"
)

type contextKey int

const (
	ctxKeyUserID  contextKey = iota
	ctxKeyTokenID contextKey = iota
)

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(string)
	return v
}

func tokenIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTokenID).(string)
	return v
}

type jwtClaims struct {
	Subject string `json:"sub"`
	Expiry  int64  `json:"exp"`
	Issuer  string `json:"iss"`
	TokenID string `json:"jti"`
}

func NewJWTAuthInterceptor(publicKeys *oidcadapter.PublicKeySet, issuer string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req any,
		_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		rawToken, err := extractBearerToken(ctx)
		if err != nil {
			return nil, err
		}

		claims, err := verifyJWT(rawToken, publicKeys)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		if claims.Issuer != issuer {
			return nil, status.Error(codes.Unauthenticated, "invalid issuer")
		}
		if time.Now().Unix() > claims.Expiry {
			return nil, status.Error(codes.Unauthenticated, "token expired")
		}
		if claims.Subject == "" {
			return nil, status.Error(codes.Unauthenticated, "missing sub claim")
		}

		ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, ctxKeyTokenID, claims.TokenID)
		return handler(ctx, req)
	}
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
	raw := vals[0]
	const prefix = "Bearer "
	if !strings.HasPrefix(raw, prefix) {
		return "", status.Error(codes.Unauthenticated, "authorization must use Bearer scheme")
	}
	return strings.TrimPrefix(raw, prefix), nil
}

func verifyJWT(rawToken string, publicKeys *oidcadapter.PublicKeySet) (*jwtClaims, error) {
	tok, err := jose.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, err
	}

	kid := ""
	if len(tok.Signatures) > 0 {
		kid = tok.Signatures[0].Header.KeyID
	}

	var payload []byte
	for _, k := range publicKeys.All() {
		if k.ID() != kid {
			continue
		}
		rsaPub, ok := k.Key().(*rsa.PublicKey)
		if !ok {
			continue
		}
		payload, err = tok.Verify(rsaPub)
		if err == nil {
			break
		}
	}
	if payload == nil {
		return nil, fmt.Errorf("no matching signing key for kid %q", kid)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}
