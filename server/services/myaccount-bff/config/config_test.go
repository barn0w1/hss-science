package config

import (
	"strings"
	"testing"
)

func validSource() MapSource {
	return MapSource{
		"OIDC_ISSUER":          "https://accounts.example.com",
		"CLIENT_ID":            "test-client",
		"CLIENT_SECRET":        "test-secret",
		"REDIRECT_URL":         "https://example.com/callback",
		"ACCOUNTS_GRPC_ADDR":   "accounts:50051",
		"REDIS_URL":            "redis://localhost:6379/0",
		"SESSION_KEY":          "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		"CORS_ALLOWED_ORIGINS": "https://example.com",
	}
}

func TestLoadFrom_Valid(t *testing.T) {
	cfg, err := LoadFrom(validSource())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.OIDCIssuer != "https://accounts.example.com" {
		t.Errorf("unexpected OIDCIssuer: %s", cfg.OIDCIssuer)
	}
	if cfg.ClientSecret != "test-secret" {
		t.Error("ClientSecret not set")
	}
	if cfg.SessionKey == [32]byte{} {
		t.Error("SessionKey is zero")
	}
	if cfg.SessionIdleTTL == 0 {
		t.Error("SessionIdleTTL is zero")
	}
	if cfg.SessionHardTTL == 0 {
		t.Error("SessionHardTTL is zero")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "https://example.com" {
		t.Errorf("unexpected CORSOrigins: %v", cfg.CORSOrigins)
	}
}

func TestLoadFrom_CustomPort(t *testing.T) {
	src := validSource()
	src["PORT"] = "9090"
	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
}

func TestLoadFrom_RequiredFields(t *testing.T) {
	required := []string{
		"OIDC_ISSUER", "CLIENT_ID", "CLIENT_SECRET",
		"REDIRECT_URL", "ACCOUNTS_GRPC_ADDR", "REDIS_URL",
	}
	for _, field := range required {
		t.Run("missing_"+field, func(t *testing.T) {
			src := validSource()
			delete(src, field)
			_, err := LoadFrom(src)
			if err == nil {
				t.Fatalf("expected error for missing %s", field)
			}
		})
	}
}

func TestLoadFrom_SessionKey_Missing(t *testing.T) {
	src := validSource()
	delete(src, "SESSION_KEY")
	_, err := LoadFrom(src)
	if err == nil || !strings.Contains(err.Error(), "SESSION_KEY") {
		t.Fatalf("expected SESSION_KEY error, got %v", err)
	}
}

func TestLoadFrom_SessionKey_NotHex(t *testing.T) {
	src := validSource()
	src["SESSION_KEY"] = "not-valid-hex-string"
	_, err := LoadFrom(src)
	if err == nil || !strings.Contains(err.Error(), "SESSION_KEY") {
		t.Fatalf("expected SESSION_KEY hex error, got %v", err)
	}
}

func TestLoadFrom_SessionKey_WrongLength(t *testing.T) {
	src := validSource()
	src["SESSION_KEY"] = "deadbeef"
	_, err := LoadFrom(src)
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected 32-byte error, got %v", err)
	}
}

func TestLoadFrom_IdleTTL_OutOfRange(t *testing.T) {
	src := validSource()
	src["SESSION_IDLE_TTL_MINUTES"] = "9999"
	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for out-of-range idle TTL")
	}
}

func TestLoadFrom_IdleTTL_NotNumeric(t *testing.T) {
	src := validSource()
	src["SESSION_IDLE_TTL_MINUTES"] = "nope"
	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for non-numeric idle TTL")
	}
}

func TestLoadFrom_HardTTL_OutOfRange(t *testing.T) {
	src := validSource()
	src["SESSION_HARD_TTL_DAYS"] = "999"
	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for out-of-range hard TTL")
	}
}

func TestLoadFrom_CORS_Missing(t *testing.T) {
	src := validSource()
	delete(src, "CORS_ALLOWED_ORIGINS")
	_, err := LoadFrom(src)
	if err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("expected CORS error, got %v", err)
	}
}

func TestLoadFrom_CORS_MultipleOrigins(t *testing.T) {
	src := validSource()
	src["CORS_ALLOWED_ORIGINS"] = "https://a.example.com, https://b.example.com"
	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(cfg.CORSOrigins))
	}
}
