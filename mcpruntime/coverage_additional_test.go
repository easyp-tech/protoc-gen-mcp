package mcpruntime

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRuntimeHelpersAndOptions(t *testing.T) {
	got := ResolveOptions(" default.ns ", []RegisterOption{nil, WithNamespace(" custom.ns ")})
	if got.Namespace != "custom.ns" {
		t.Fatalf("ResolveOptions namespace = %q, want custom.ns", got.Namespace)
	}
	if v := *Ptr(42); v != 42 {
		t.Fatalf("Ptr(42) = %d, want 42", v)
	}

	var buf bytes.Buffer
	if err := writeSSERetry(&buf, 0); err != nil || buf.Len() != 0 {
		t.Fatalf("writeSSERetry(0) = %q, %v", buf.String(), err)
	}
	if err := writeSSERetry(&buf, 1500); err != nil {
		t.Fatal(err)
	}
	if err := writeSSEComment(&buf, "ping"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "retry: 1500") || !strings.Contains(buf.String(), ": ping") {
		t.Fatalf("unexpected SSE helpers output: %q", buf.String())
	}

	if ms, ok := parseRetryMS("retry: 2500"); !ok || ms != 2500 {
		t.Fatalf("parseRetryMS = %d, %v", ms, ok)
	}
	if _, ok := parseRetryMS("data: nope"); ok {
		t.Fatal("parseRetryMS accepted non-retry line")
	}
	if _, ok := parseRetryMS("retry: nope"); ok {
		t.Fatal("parseRetryMS accepted invalid integer")
	}

	if !acceptIncludes("application/*;q=0.9", contentTypeJSON) {
		t.Fatal("application/* should accept application/json")
	}
	if !acceptIncludes("*/*", contentTypeSSE) {
		t.Fatal("*/* should accept text/event-stream")
	}
	if acceptIncludes("", contentTypeJSON) {
		t.Fatal("empty Accept should reject media type")
	}

	rec := httptest.NewRecorder()
	writeSSEHeaders(rec)
	if rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("unexpected SSE headers: %v", rec.Header())
	}
}

func TestOriginAndProtocolHelpers(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "http://127.0.0.1", "http://[::1]:8080"} {
		if !isLocalhostOrigin(origin) {
			t.Fatalf("localhost origin rejected: %s", origin)
		}
	}
	for _, origin := range []string{"https://example.com", "://bad"} {
		if isLocalhostOrigin(origin) {
			t.Fatalf("non-local origin accepted: %s", origin)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !validateOrigin(req, nil, false) {
		t.Fatal("request without Origin should be accepted")
	}
	req.Header.Set("Origin", "https://example.com/")
	if !validateOrigin(req, []string{"https://EXAMPLE.com"}, false) {
		t.Fatal("configured origin should be accepted case-insensitively")
	}
	if !validateOrigin(req, nil, true) {
		t.Fatal("AllowAllOrigins should accept origin")
	}

	if got, err := resolveProtocolVersion("", nil); err != nil || got != "2025-03-26" {
		t.Fatalf("default protocol = %q, %v", got, err)
	}
	sess := NewSessionWithID("protocol-test")
	if !sess.markReady("2025-11-25") {
		t.Fatal("markReady unexpectedly failed")
	}
	if got, err := resolveProtocolVersion("", sess); err != nil || got != "2025-11-25" {
		t.Fatalf("session protocol = %q, %v", got, err)
	}
	if _, err := resolveProtocolVersion("1999-01-01", sess); err == nil || !strings.Contains(err.Error(), "1999-01-01") {
		t.Fatalf("unsupported protocol error = %v", err)
	}
}

func TestSessionManagerLifecycle(t *testing.T) {
	m := newSessionManager(time.Second)
	s := m.Create()
	if m.Len() != 1 {
		t.Fatalf("Len = %d, want 1", m.Len())
	}
	if m.Get("") != nil || m.Get("missing") != nil {
		t.Fatal("missing session lookup should return nil")
	}
	if m.Get(s.ID) != s {
		t.Fatal("Get did not return created session")
	}

	s.mu.Lock()
	s.lastActive = time.Now().Add(-2 * time.Second)
	s.mu.Unlock()
	m.reapExpired()
	if m.Len() != 0 {
		t.Fatalf("expired session was not reaped: %d", m.Len())
	}
	if m.Delete("missing") {
		t.Fatal("Delete(missing) should report false")
	}

	s2 := m.Create()
	if !m.Delete(s2.ID) {
		t.Fatal("Delete(existing) should report true")
	}
	_ = m.Create()
	m.Close()
	m.Close()
	if m.Len() != 0 {
		t.Fatalf("Close left sessions behind: %d", m.Len())
	}
}

func TestSessionAndStreamLifecycle(t *testing.T) {
	s := NewSessionWithID("")
	if s.ID == "" || s.CreatedAt().IsZero() || s.LastActive().IsZero() {
		t.Fatal("session timestamps/id were not initialized")
	}
	if s.ProtocolVersion() != "" || s.IsReady() {
		t.Fatal("new session should not be initialized")
	}
	s.streams = nil
	if err := s.SendNotification("notifications/test", nil); err != nil {
		t.Fatalf("SendNotification without streams: %v", err)
	}

	st := newMessageStream("listen1", 2)
	sub := st.subscribe(0)
	id1, err := st.publish([]byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = st.publish([]byte("two"))
	id3, _ := st.publish([]byte("three"))
	if len(st.buffer) != 2 {
		t.Fatalf("buffer len = %d, want 2", len(st.buffer))
	}
	if ev := <-sub; ev.ID != id1 {
		t.Fatalf("subscriber first event = %q, want %q", ev.ID, id1)
	}
	st.unsubscribe(sub)
	events, err := st.eventsAfter(id1)
	if err != nil || len(events) != 2 || events[1].ID != id3 {
		t.Fatalf("eventsAfter = %#v, %v", events, err)
	}
	if _, err := st.eventsAfter("bad"); err == nil {
		t.Fatal("eventsAfter accepted invalid id")
	}
	if _, err := st.eventsAfter("other_1"); err == nil {
		t.Fatal("eventsAfter accepted another stream id")
	}
	st.close()
	st.close()
	if _, err := st.publish([]byte("closed")); err == nil {
		t.Fatal("publish on closed stream should fail")
	}

	ss := newSessionStreams(1)
	ss.setBufferSize(3)
	listen := ss.openStream("listen")
	if again := ss.openStream("listen"); again != listen {
		t.Fatal("listen stream was not reused")
	}
	if ss.getStream(listen.id) != listen {
		t.Fatal("getStream did not return listen stream")
	}
	post := ss.openStream("post")
	ss.closeStream(post.id)
	ss.closeStream("missing")
	ss.closeAll()
}

func TestStreamableHTTPValidationBranches(t *testing.T) {
	t.Run("nil server panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil server")
			}
		}()
		_ = NewStreamableHTTPHandler(nil, StreamableHTTPOptions{})
	})

	server := NewServer("coverage", "v1")
	h := NewStreamableHTTPHandler(server, StreamableHTTPOptions{
		AllowAllOrigins:   true,
		Authorize:         func(*http.Request) error { return context.Canceled },
		HeartbeatInterval: -1,
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("authorize status = %d, want 401", rec.Code)
	}

	_, h = newHTTPTestServer(t, StreamableHTTPOptions{})
	cases := []struct {
		name   string
		method string
		body   string
		accept string
		want   int
	}{
		{"method", http.MethodPut, "", "", http.StatusMethodNotAllowed},
		{"bad accept", http.MethodPost, `{}`, contentTypeJSON, http.StatusBadRequest},
		{"empty body", http.MethodPost, "", "application/json, text/event-stream", http.StatusBadRequest},
		{"batch", http.MethodPost, `[]`, "application/json, text/event-stream", http.StatusBadRequest},
		{"invalid json", http.MethodPost, `{`, "application/json, text/event-stream", http.StatusBadRequest},
		{"unclassified json", http.MethodPost, `{}`, "application/json, text/event-stream", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/mcp", strings.NewReader(tc.body))
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	req = httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("DELETE without session = %d, want 400", rec.Code)
	}
}

func TestServeStreamableHTTPCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := NewServer("coverage", "v1")
	if err := ServeStreamableHTTP(ctx, "127.0.0.1:0", server, StreamableHTTPOptions{}); err != nil {
		t.Fatalf("ServeStreamableHTTP canceled context: %v", err)
	}
}
