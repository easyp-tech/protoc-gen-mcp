package mcpruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

func TestJWKSVerifier_ClaimsAndScopes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := map[string]any{"keys": []any{map[string]any{
		"kty": "RSA", "kid": "primary", "alg": "RS256", "use": "sig",
		"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jwks); err != nil {
			t.Errorf("JWKS encode: %v", err)
		}
	}))
	defer remote.Close()
	aud := "https://mcp.example.com/mcp"
	verify, err := NewJWKSVerifier(JWKSVerifierConfig{
		JWKSURL: remote.URL + "/jwks", Issuer: remote.URL,
		Audience: aud, HTTPClient: remote.Client(),
	})
	if err != nil {
		t.Fatalf("NewJWKSVerifier: %v", err)
	}
	sign := func(claims jwt.MapClaims, kid string) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = kid
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatalf("sign JWT: %v", err)
		}
		return signed
	}
	valid := jwt.MapClaims{
		"iss": remote.URL, "aud": aud, "sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"scope": "reports:read reports:write",
	}
	info, err := verify(context.Background(), sign(valid, "primary"), nil)
	if err != nil {
		t.Fatalf("verify valid JWT: %v", err)
	}
	if info.UserID != "user-123" || !slices.Equal(info.Scopes, []string{"reports:read", "reports:write"}) {
		t.Fatalf("TokenInfo = %+v", info)
	}
	for _, tc := range []struct {
		name   string
		claims jwt.MapClaims
		kid    string
	}{
		{"wrong issuer", jwt.MapClaims{"iss": "https://other.example.com", "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}, "primary"},
		{"wrong audience", jwt.MapClaims{"iss": remote.URL, "aud": "https://another.example.com", "exp": time.Now().Add(time.Hour).Unix()}, "primary"},
		{"expired", jwt.MapClaims{"iss": remote.URL, "aud": aud, "exp": time.Now().Add(-time.Hour).Unix()}, "primary"},
		{"unknown kid", valid, "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := verify(context.Background(), sign(tc.claims, tc.kid), nil)
			if err == nil || err != auth.ErrInvalidToken {
				t.Fatalf("JWT error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestJWKSVerifier_InvalidConfiguration(t *testing.T) {
	for _, tc := range []JWKSVerifierConfig{
		{}, {JWKSURL: "http://insecure.example/jwks", Issuer: "https://auth.example", Audience: "resource"},
		{JWKSURL: "https://auth.example/jwks", Issuer: "http://auth.example", Audience: "resource"},
		{JWKSURL: "https://auth.example/jwks", Issuer: "https://auth.example"},
	} {
		if _, err := NewJWKSVerifier(tc); err == nil {
			t.Errorf("accepted invalid verifier configuration %+v", tc)
		}
	}
}

func TestJWKSVerifier_RejectHTTPRedirect(t *testing.T) {
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL+"/jwks", http.StatusFound)
	}))
	defer secure.Close()
	cache := &cachedJWKS{
		client: secure.Client(),
		config: JWKSVerifierConfig{JWKSURL: secure.URL+"/jwks"},
	}
	err := cache.load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "insecure scheme") {
		t.Fatalf("HTTPS-to-HTTP JWKS redirect error=%v; want secure refusal", err)
	}
}

func TestJWKSVerifier_UnknownKidRefreshIsThrottled(t *testing.T) {
	var requests int
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil { t.Fatal(err) }
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{"keys":[]any{map[string]any{
			"kty":"RSA","kid":"known","alg":"RS256","use":"sig",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	}))
	defer remote.Close()
	cache := &cachedJWKS{client: remote.Client(),config: JWKSVerifierConfig{JWKSURL: remote.URL}}
	// Prime the cache, then emulate a cache which is more than a minute old.
	if err := cache.load(context.Background()); err != nil { t.Fatal(err) }
	cache.expires = time.Now().Add(3*time.Minute)
	for i:=0; i<8; i++ {
		if _,err:=cache.key(context.Background(), "unknown-kid"); err==nil {
			t.Fatal("unknown kid was accepted")
		}
	}
	if requests != 2 {
		t.Fatalf("unknown kid caused %d JWKS requests; expected 1 initial and 1 throttled refresh", requests)
	}
}
