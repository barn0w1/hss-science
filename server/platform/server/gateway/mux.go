package gateway

import (
	"context"
	"net/http"
	"strconv"

	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// NewMux returns a standard ServeMux for the platform.
// It enforces consistent JSON serialization rules and handles metadata translation.
func NewMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		// JSON設定: proto定義のsnake_caseを維持し、空フィールドもレスポンスに含める
		runtime.WithMarshalerOption(runtime.MuxDescriptor, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		// HTTPヘッダー -> gRPCメタデータの変換ルール (IPやUAを通すため)
		runtime.WithIncomingHeaderMatcher(customHeaderMatcher),
		// gRPCメタデータ -> HTTPヘッダーへの変換フック (Cookie, Redirect用)
		runtime.WithForwardResponseOption(httpResponseModifier),
	)
}

// customHeaderMatcher は、HTTPヘッダーをgRPCメタデータに変換する際のルールを定義します。
// デフォルトでは変換されない重要なヘッダー(IPアドレス等)を通過させます。
func customHeaderMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case "x-forwarded-for", "x-real-ip":
		return key, true
	case "user-agent", "x-user-agent":
		return key, true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}

// httpResponseModifier は gRPC メタデータを HTTP レスポンスヘッダーに変換します。
// gRPCの制限(CookieやRedirectがない)をGateway側で補完する役割を持ちます。
func httpResponseModifier(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	// 1. Cookie処理: Metadata "set-cookie" -> HTTP Header "Set-Cookie"
	if cookies := md.HeaderMD.Get("set-cookie"); len(cookies) > 0 {
		for _, cookie := range cookies {
			w.Header().Add("Set-Cookie", cookie)
		}
	}

	// 2. リダイレクト/ステータスコード処理: Metadata "x-http-code" -> HTTP Status Code
	if codes := md.HeaderMD.Get("x-http-code"); len(codes) > 0 {
		if code, err := strconv.Atoi(codes[0]); err == nil {
			w.WriteHeader(code)
		}
	}

	// Locationヘッダーはruntimeが自動処理する場合もありますが、念のため明示的にサポート
	if locs := md.HeaderMD.Get("location"); len(locs) > 0 {
		w.Header().Set("Location", locs[0])
	}

	return nil
}

// WithCORS wraps an http.Handler to support Cross-Origin Resource Sharing.
func WithCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 注意: 本番環境では環境変数等で許可オリジンを絞るべきです
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-Agent, Cache-Control")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}
