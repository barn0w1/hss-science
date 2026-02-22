package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	hashKey := []byte("01234567890123456789012345678901") // 32 bytes
	blockKey := []byte("0123456789012345")                // 16 bytes
	mgr := New(hashKey, blockKey, 3600, false)

	data := &Data{
		UserID:   "user-123",
		IssuedAt: time.Now().Unix(),
	}

	// Encode.
	cookieStr, err := mgr.Encode(data)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if cookieStr == "" {
		t.Fatal("expected non-empty cookie string")
	}

	// Build a request with the cookie.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookieStr)

	// Decode.
	decoded, err := mgr.Decode(req)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded == nil {
		t.Fatal("expected non-nil decoded data")
	}
	if decoded.UserID != data.UserID {
		t.Fatalf("expected UserID %s, got %s", data.UserID, decoded.UserID)
	}
	if decoded.IssuedAt != data.IssuedAt {
		t.Fatalf("expected IssuedAt %d, got %d", data.IssuedAt, decoded.IssuedAt)
	}
}

func TestDecodeNoCookie(t *testing.T) {
	hashKey := []byte("01234567890123456789012345678901")
	blockKey := []byte("0123456789012345")
	mgr := New(hashKey, blockKey, 3600, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	data, err := mgr.Decode(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatal("expected nil data for absent cookie")
	}
}

func TestDecodeTamperedCookie(t *testing.T) {
	hashKey := []byte("01234567890123456789012345678901")
	blockKey := []byte("0123456789012345")
	mgr := New(hashKey, blockKey, 3600, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "accounts_session",
		Value: "tampered-value",
	})

	_, err := mgr.Decode(req)
	if err == nil {
		t.Fatal("expected error for tampered cookie")
	}
}

func TestClearCookie(t *testing.T) {
	hashKey := []byte("01234567890123456789012345678901")
	blockKey := []byte("0123456789012345")
	mgr := New(hashKey, blockKey, 3600, false)

	clear := mgr.ClearCookie()
	if clear == "" {
		t.Fatal("expected non-empty clear cookie string")
	}

	// Parse to verify MaxAge=-1.
	header := http.Header{}
	header.Add("Set-Cookie", clear)
	resp := http.Response{Header: header}
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookies[0].MaxAge)
	}
}

func TestIsValid(t *testing.T) {
	hashKey := []byte("01234567890123456789012345678901")
	blockKey := []byte("0123456789012345")
	mgr := New(hashKey, blockKey, 3600, false)

	t.Run("valid", func(t *testing.T) {
		data := &Data{UserID: "user-1", IssuedAt: time.Now().Unix()}
		if !mgr.IsValid(data) {
			t.Fatal("expected valid session")
		}
	})

	t.Run("nil", func(t *testing.T) {
		if mgr.IsValid(nil) {
			t.Fatal("expected invalid for nil")
		}
	})

	t.Run("expired", func(t *testing.T) {
		data := &Data{UserID: "user-1", IssuedAt: time.Now().Add(-2 * time.Hour).Unix()}
		if mgr.IsValid(data) {
			t.Fatal("expected invalid for expired session")
		}
	})
}
