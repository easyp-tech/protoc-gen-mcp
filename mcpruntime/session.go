package mcpruntime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type sessionContextKey struct{}

// Session holds per-client MCP protocol lifecycle state.
// Tool/prompt/resource registries live on Server and are shared across sessions.
type Session struct {
	ID string

	mu              sync.Mutex
	ready           bool
	protocolVersion string
	createdAt       time.Time
	lastActive      time.Time

	// streams holds SSE stream state for Streamable HTTP (may be nil for stdio).
	streams *sessionStreams
}

// NewSession creates a new protocol session with a cryptographically random ID.
// The ID uses only visible ASCII characters (MCP session ID requirements).
func NewSession() *Session {
	now := time.Now()
	return &Session{
		ID:         generateSessionID(),
		createdAt:  now,
		lastActive: now,
		streams:    newSessionStreams(0),
	}
}

// NewSessionWithID creates a session with a fixed ID (tests / stdio).
func NewSessionWithID(id string) *Session {
	if id == "" {
		id = generateSessionID()
	}
	now := time.Now()
	return &Session{
		ID:         id,
		createdAt:  now,
		lastActive: now,
		streams:    newSessionStreams(0),
	}
}

// WithSession attaches a session to the context for HandleRaw dispatch.
func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

// SessionFromContext returns the session attached to ctx, or nil.
func SessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey{}).(*Session)
	return s
}

// IsReady reports whether initialize completed for this session.
func (s *Session) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready
}

// ProtocolVersion returns the negotiated protocol version, if any.
func (s *Session) ProtocolVersion() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.protocolVersion
}

// markReady marks the session as initialized and stores the negotiated version.
// Returns false if the session was already ready.
func (s *Session) markReady(protocolVersion string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready {
		return false
	}
	s.ready = true
	s.protocolVersion = protocolVersion
	s.lastActive = time.Now()
	return true
}

// touch updates last-active time.
func (s *Session) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActive = time.Now()
}

// LastActive returns the last activity timestamp.
func (s *Session) LastActive() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastActive
}

// CreatedAt returns session creation time.
func (s *Session) CreatedAt() time.Time {
	return s.createdAt
}

// Close tears down session streams (HTTP SSE).
func (s *Session) Close() {
	if s.streams != nil {
		s.streams.closeAll()
	}
}

// SendNotification pushes a JSON-RPC notification to GET SSE listeners.
// No-op when no streams are attached (stdio).
func (s *Session) SendNotification(method string, params any) error {
	if s.streams == nil {
		return nil
	}
	return s.streams.broadcastNotification(method, params)
}

func generateSessionID() string {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to timestamp-based id.
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
