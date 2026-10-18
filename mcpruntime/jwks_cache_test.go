package mcpruntime

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type jwksRoundTripper func(*http.Request) (*http.Response, error)

func (f jwksRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cacheTestJWKS(kid string) string {
	// Cache tests only parse public keys. JWT signature verification is covered
	// separately with a real generated RSA key.
	n := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2047), big.NewInt(1))
	return fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","n":%q,"e":"AQAB"}]}`, kid, base64.RawURLEncoding.EncodeToString(n.Bytes()))
}

func cacheTestResponse(r *http.Request, body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}

func TestJWKSCache_ExpiredFailureBackoffAndRecovery(t *testing.T) {
	var requests atomic.Int32
	var unavailable atomic.Bool
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if unavailable.Load() {
			http.Error(w, "IdP unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, cacheTestJWKS("known"))
	}))
	defer remote.Close()
	now := time.Now()
	cache := &cachedJWKS{client: remote.Client(), config: JWKSVerifierConfig{JWKSURL: remote.URL}, now: func() time.Time { return now }}
	if _, err := cache.key(context.Background(), "known"); err != nil {
		t.Fatal(err)
	}
	// At the exact expiry boundary, retained keys must not authenticate.
	now = now.Add(jwksCacheTTL)
	unavailable.Store(true)
	if key, err := cache.key(context.Background(), "known"); err == nil || key != nil {
		t.Fatalf("expired key accepted after failed refresh: key=%v err=%v", key, err)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if key, err := cache.key(context.Background(), "known"); err == nil || key != nil {
				t.Error("expired key accepted during retry backoff")
			}
		})
	}
	wg.Wait()
	if got := requests.Load(); got != 2 {
		t.Fatalf("got %d requests; want one initial fetch and one failed refresh", got)
	}
	unavailable.Store(false)
	now = now.Add(jwksRetryMin)
	if _, err := cache.key(context.Background(), "known"); err != nil {
		t.Fatalf("cache did not recover after retry deadline: %v", err)
	}
	if requests.Load() != 3 || cache.retryDelay != 0 || !cache.retryAt.IsZero() || cache.refreshErr != nil {
		t.Fatalf("successful refresh did not reset backoff: requests=%d delay=%v retryAt=%v err=%v", requests.Load(), cache.retryDelay, cache.retryAt, cache.refreshErr)
	}
}

func TestJWKSCache_BackoffIsExponentialAndBounded(t *testing.T) {
	now := time.Now()
	var requests int
	cache := &cachedJWKS{
		config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"},
		now:    func() time.Time { return now },
		client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
			requests++
			return cacheTestResponse(r, "unavailable", http.StatusServiceUnavailable), nil
		})},
	}
	for i, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, time.Minute, time.Minute} {
		if _, err := cache.key(context.Background(), "known"); err == nil {
			t.Fatal("unavailable JWKS accepted")
		}
		if cache.retryDelay != delay || cache.retryAt.Sub(now) != delay {
			t.Fatalf("attempt %d delay=%v deadline=%v; want %v", i, cache.retryDelay, cache.retryAt.Sub(now), delay)
		}
		now = now.Add(delay - time.Nanosecond)
		if _, err := cache.key(context.Background(), "another-kid"); err == nil {
			t.Fatal("retry backoff bypassed")
		}
		if requests != i+1 {
			t.Fatalf("attempt %d made %d requests during backoff", i, requests)
		}
		now = now.Add(time.Nanosecond)
	}
}

func TestJWKSCache_ConcurrentRefreshAndWaiterCancellation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("expired=%v", expired), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			var requests atomic.Int32
			cache := &cachedJWKS{
				config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"},
				client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
					if requests.Add(1) == 1 {
						close(started)
					}
					<-release
					return cacheTestResponse(r, cacheTestJWKS("known"), http.StatusOK), nil
				})},
			}
			if expired {
				cache.keys = map[string]*rsa.PublicKey{"known": {}}
				cache.expires = time.Now().Add(-time.Second)
			}
			var wg sync.WaitGroup
			for range 32 {
				wg.Go(func() {
					if _, err := cache.key(context.Background(), "known"); err != nil {
						t.Errorf("refresh: %v", err)
					}
				})
			}
			<-started
			ctx, cancel := context.WithCancel(context.Background())
			waiter := make(chan error, 1)
			go func() { _, err := cache.key(ctx, "known"); waiter <- err }()
			cancel()
			select {
			case err := <-waiter:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("canceled waiter: %v", err)
				}
			case <-time.After(time.Second):
				t.Error("canceled waiter blocked behind JWKS network request")
			}
			releaseOnce.Do(func() { close(release) })
			wg.Wait()
			if got := requests.Load(); got != 1 {
				t.Fatalf("concurrent refresh made %d requests; want 1", got)
			}
		})
	}
}

func TestJWKSCache_FreshKeysDoNotWaitForRotationRefresh(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	known := &rsa.PublicKey{}
	cache := &cachedJWKS{
		keys: map[string]*rsa.PublicKey{"known": known}, expires: time.Now().Add(3 * time.Minute),
		config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"},
		client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
			close(started)
			<-release
			return cacheTestResponse(r, cacheTestJWKS("rotated"), http.StatusOK), nil
		})},
	}
	rotation := make(chan error, 1)
	go func() { _, err := cache.key(context.Background(), "rotated"); rotation <- err }()
	<-started
	lookup := make(chan error, 1)
	go func() {
		key, err := cache.key(context.Background(), "known")
		if err == nil && key != known {
			err = errors.New("wrong fresh key")
		}
		lookup <- err
	}()
	select {
	case err := <-lookup:
		if err != nil {
			t.Errorf("fresh key: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("fresh key lookup blocked behind JWKS network request")
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-rotation; err != nil {
		t.Fatalf("rotated key: %v", err)
	}
	if _, err := cache.key(context.Background(), "known"); err == nil {
		t.Fatal("key removed by refreshed JWKS was still accepted")
	}
}

func TestJWKSCache_UnknownKidFailureRemainsThrottled(t *testing.T) {
	now := time.Now()
	var requests int
	cache := &cachedJWKS{
		keys: map[string]*rsa.PublicKey{"known": {}}, expires: now.Add(3 * time.Minute),
		config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"}, now: func() time.Time { return now },
		client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
			requests++
			return cacheTestResponse(r, "unavailable", http.StatusServiceUnavailable), nil
		})},
	}
	for i := range 60 {
		if _, err := cache.key(context.Background(), fmt.Sprintf("unknown-%d", i)); err == nil {
			t.Fatal("unknown key accepted")
		}
		if _, err := cache.key(context.Background(), "known"); err != nil {
			t.Fatalf("fresh known key rejected: %v", err)
		}
		now = now.Add(time.Second)
	}
	if requests != 1 {
		t.Fatalf("unknown kids made %d requests before minute elapsed; want 1", requests)
	}
	if _, err := cache.key(context.Background(), "unknown-next"); err == nil {
		t.Fatal("unknown key accepted")
	}
	if requests != 2 {
		t.Fatalf("unknown kid did not retry after minute: requests=%d", requests)
	}
}

func TestJWKSCache_CustomClientStillHasFetchDeadline(t *testing.T) {
	cache := &cachedJWKS{
		config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"},
		client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > jwksRefreshTimeout {
				t.Error("JWKS fetch has no bounded deadline")
			}
			return nil, context.DeadlineExceeded
		})},
	}
	if _, err := cache.key(context.Background(), "known"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error=%v", err)
	}
}

func TestJWKSCache_RefreshOwnerCancellationDoesNotPoisonCache(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var requests atomic.Int32
	cache := &cachedJWKS{
		config: JWKSVerifierConfig{JWKSURL: "https://auth.example/jwks"},
		client: &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
			if requests.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
				return cacheTestResponse(r, cacheTestJWKS("known"), http.StatusOK), nil
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		})},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := make(chan error, 1)
	go func() { _, err := cache.key(ctx, "known"); owner <- err }()
	<-started
	cancel()
	select {
	case err := <-owner:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled refresh owner: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh owner could not cancel independently")
	}
	waiter := make(chan error, 1)
	go func() { _, err := cache.key(context.Background(), "known"); waiter <- err }()
	releaseOnce.Do(func() { close(release) })
	if err := <-waiter; err != nil {
		t.Fatalf("owner cancellation poisoned another request: %v", err)
	}
	if requests.Load() != 1 || cache.refreshErr != nil || cache.retryDelay != 0 {
		t.Fatalf("owner cancellation affected shared fetch: requests=%d err=%v delay=%v", requests.Load(), cache.refreshErr, cache.retryDelay)
	}
}
