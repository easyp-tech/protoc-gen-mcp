package mcpruntime

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// JWKSVerifierConfig supplies public OAuth policy, never tokens or secrets.
type JWKSVerifierConfig struct {
	JWKSURL, Issuer, Audience string
	HTTPClient                *http.Client // Optional trusted client, useful for test CAs.
}

type cachedJWKS struct {
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
	lastUnknownKeyRefresh time.Time
	client  *http.Client
	config  JWKSVerifierConfig
}

// NewJWKSVerifier returns an OAuth bearer TokenVerifier for RS256 access JWTs.
// Other signing algorithms are deliberately not accepted. Invalid claims,
// missing kid, weak RSA keys, unknown issuers and audiences are rejected.
func NewJWKSVerifier(c JWKSVerifierConfig) (auth.TokenVerifier, error) {
	if err := requireHTTPSURL("JWKS URL", c.JWKSURL); err != nil {
		return nil, err
	}
	if err := requireHTTPSURL("issuer", c.Issuer); err != nil {
		return nil, err
	}
	if c.Audience == "" {
		return nil, errors.New("mcpruntime: JWT audience is required")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	cache := &cachedJWKS{client: client, config: c}
	return func(ctx context.Context, raw string, _ *http.Request) (*auth.TokenInfo, error) {
		claims := jwt.MapClaims{}
		parser := jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(c.Issuer),
			jwt.WithAudience(c.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(10*time.Second),
		)
		token, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			kid, ok := t.Header["kid"].(string)
			if !ok || kid == "" {
				return nil, errors.New("missing JWT key id")
			}
			key, err := cache.key(ctx, kid)
			if err != nil {
				return nil, err
			}
			return key, nil
		})
		if err != nil || token == nil || !token.Valid {
			return nil, auth.ErrInvalidToken
		}
		exp, err := claims.GetExpirationTime()
		if err != nil || exp == nil {
			return nil, auth.ErrInvalidToken
		}
		sub, _ := claims.GetSubject()
		var scopes []string
		if scope, ok := claims["scope"].(string); ok {
			scopes = strings.Fields(scope)
		} else if a, ok := claims["scp"].([]any); ok {
			for _, value := range a {
				str, ok := value.(string)
				if !ok {
					return nil, auth.ErrInvalidToken
				}
				scopes = append(scopes, str)
			}
		} else if a, ok := claims["scp"].(string); ok {
			scopes = strings.Fields(a)
		}
		return &auth.TokenInfo{Scopes: scopes, Expiration: exp.Time, UserID: sub}, nil
	}, nil
}

func requireHTTPSURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Fragment != "" || u.User != nil {
		return fmt.Errorf("mcpruntime: %s must be a valid HTTPS URL", field)
	}
	return nil
}

func (c *cachedJWKS) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.keys == nil || now.After(c.expires) {
		if err := c.load(ctx); err != nil {
			return nil, err
		}
	}
	if pub, ok := c.keys[kid]; ok {
		return pub, nil
	}
	// Refresh keys for provider rotation, but only after the cache's minimum
	// refresh interval; unknown kid must not trigger unlimited network calls.
	if c.expires.Sub(now) < 4*time.Minute && now.Sub(c.lastUnknownKeyRefresh) >= time.Minute {
		// Rate-limit refresh requests triggered by attacker-controlled kid
		// values, including unsuccessful JWKS refresh attempts.
		c.lastUnknownKeyRefresh = now
		if err := c.load(ctx); err != nil {
			return nil, err
		}
	}
	if pub, ok := c.keys[kid]; ok {
		return pub, nil
	}
	return nil, errors.New("unknown JWT key id")
}

func (c *cachedJWKS) load(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Even a custom HTTP client must not silently downgrade JWKS fetches.
	if resp.Request != nil && resp.Request.URL.Scheme != "https" {
		return errors.New("JWKS redirect to an insecure scheme is forbidden")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("JWKS response exceeds 1 MiB")
	}
	var result struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(result.Keys))
	for _, k := range result.Keys {
		if k.Kid == "" || k.Kty != "RSA" || (k.Alg != "" && k.Alg != "RS256") || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		exp, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil || len(exp) == 0 || len(exp) > 4 {
			continue
		}
		e := 0
		for _, b := range exp {
			e = e<<8 | int(b)
		}
		modulus := new(big.Int).SetBytes(n)
		if modulus.BitLen() < 2048 || e < 3 || e%2 == 0 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: modulus, E: e}
	}
	if len(keys) == 0 {
		return errors.New("JWKS response contains no usable RS256 signing keys")
	}
	c.keys = keys
	c.expires = time.Now().Add(5 * time.Minute)
	return nil
}
