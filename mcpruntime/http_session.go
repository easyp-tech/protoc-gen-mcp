package mcpruntime

import (
	"sync"
	"time"
)

const defaultSessionTTL = 30 * time.Minute

// sessionManager tracks Streamable HTTP sessions with idle TTL.
type sessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
	stopCh   chan struct{}
	stopped  bool
}

func newSessionManager(ttl time.Duration) *sessionManager {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	m := &sessionManager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	go m.reaper()
	return m
}

func (m *sessionManager) Create() *Session {
	s := NewSession()
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

func (m *sessionManager) Get(id string) *Session {
	if id == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil
	}
	s.touch()
	return s
}

func (m *sessionManager) Delete(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if s != nil {
		s.Close()
	}
	return ok
}

func (m *sessionManager) Close() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)
	sessions := make([]*Session, 0, len(m.sessions))
	for id, s := range m.sessions {
		sessions = append(sessions, s)
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		s.Close()
	}
}

func (m *sessionManager) reaper() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.reapExpired()
		}
	}
}

func (m *sessionManager) reapExpired() {
	deadline := time.Now().Add(-m.ttl)
	m.mu.Lock()
	var expired []*Session
	for id, s := range m.sessions {
		if s.LastActive().Before(deadline) {
			expired = append(expired, s)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, s := range expired {
		s.Close()
	}
}

// Len returns the number of active sessions (tests).
func (m *sessionManager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
