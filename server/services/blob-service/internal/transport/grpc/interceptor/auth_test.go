package interceptor_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/barn0w1/hss-science/server/gen/blob/v1"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/transport/grpc/interceptor"
)

const bufSize = 1024 * 1024

type oidcTestServer struct {
	key    *rsa.PrivateKey
	keyID  string
	issuer string
	srv    *httptest.Server
}

func newOIDCTestServer(t *testing.T) *oidcTestServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	ots := &oidcTestServer{key: key, keyID: "test-key-1"}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ots.srv = srv
	ots.issuer = srv.URL

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   ots.issuer,
			"jwks_uri": ots.issuer + "/.well-known/jwks.json",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		jwks := jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, KeyID: ots.keyID, Algorithm: "RS256", Use: "sig"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	return ots
}

func (ots *oidcTestServer) signToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: ots.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", ots.keyID),
	)
	require.NoError(t, err)

	raw, err := josejwt.Signed(sig).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func validClaims(issuer string) map[string]any {
	return map[string]any{
		"iss": issuer,
		"aud": []string{"blob-service"},
		"sub": "drive-service",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

func setupAuthServer(t *testing.T, ots *oidcTestServer) (pb.BlobServiceClient, func()) {
	t.Helper()

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, ots.issuer)
	require.NoError(t, err)

	auth := interceptor.NewAuthInterceptor(provider, "blob-service")

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(auth.Unary()),
	)
	pb.RegisterBlobServiceServer(srv, &noopBlobServer{})

	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	return pb.NewBlobServiceClient(conn), func() {
		_ = conn.Close()
		srv.GracefulStop()
	}
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	ots := newOIDCTestServer(t)
	client, cleanup := setupAuthServer(t, ots)
	defer cleanup()

	token := ots.signToken(t, validClaims(ots.issuer))
	ctx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "+token))

	_, err := client.GetBlobInfo(ctx, &pb.GetBlobInfoRequest{BlobId: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"})
	st, _ := status.FromError(err)
	assert.NotEqual(t, codes.Unauthenticated, st.Code(), "valid token should not get Unauthenticated")
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	ots := newOIDCTestServer(t)
	client, cleanup := setupAuthServer(t, ots)
	defer cleanup()

	_, err := client.GetBlobInfo(context.Background(), &pb.GetBlobInfoRequest{BlobId: "abc"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_MalformedBearer(t *testing.T) {
	ots := newOIDCTestServer(t)
	client, cleanup := setupAuthServer(t, ots)
	defer cleanup()

	ctx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("authorization", "NotBearer token"))
	_, err := client.GetBlobInfo(ctx, &pb.GetBlobInfoRequest{BlobId: "abc"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_ExpiredToken(t *testing.T) {
	ots := newOIDCTestServer(t)
	client, cleanup := setupAuthServer(t, ots)
	defer cleanup()

	claims := validClaims(ots.issuer)
	claims["exp"] = time.Now().Add(-time.Hour).Unix()
	token := ots.signToken(t, claims)

	ctx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "+token))
	_, err := client.GetBlobInfo(ctx, &pb.GetBlobInfoRequest{BlobId: "abc"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_WrongAudience(t *testing.T) {
	ots := newOIDCTestServer(t)
	client, cleanup := setupAuthServer(t, ots)
	defer cleanup()

	claims := validClaims(ots.issuer)
	claims["aud"] = []string{"other-service"}
	token := ots.signToken(t, claims)

	ctx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "+token))
	_, err := client.GetBlobInfo(ctx, &pb.GetBlobInfoRequest{BlobId: "abc"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

type noopBlobServer struct {
	pb.UnimplementedBlobServiceServer
}

func (n *noopBlobServer) GetBlobInfo(_ context.Context, req *pb.GetBlobInfoRequest) (*pb.GetBlobInfoResponse, error) {
	return nil, status.Errorf(codes.NotFound, "blob %s not found", req.BlobId)
}
