package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	defaultHTTPPath     = "/mcp"
	maxHTTPBodyBytes    = 4 << 20 // 4 MiB
	defaultSSERetryMS   = 3000
	defaultHeartbeatInt = 15 * time.Second
)

// StreamableHTTPOptions configures the Streamable HTTP transport.
type StreamableHTTPOptions struct {
	// Path is the MCP endpoint path. Default "/mcp".
	// When using NewStreamableHTTPHandler as a bare http.Handler, path matching
	// is left to the parent mux; this field is used by ServeStreamableHTTP.
	Path string

	// AllowedOrigins is the Origin allowlist. Empty means only localhost Origins
	// are accepted when Origin is present (non-browser clients omit Origin).
	AllowedOrigins []string

	// AllowAllOrigins disables Origin checks (unsafe; development only).
	AllowAllOrigins bool

	// SessionTTL is idle session lifetime. Zero → 30m.
	SessionTTL time.Duration

	// EventBufferSize caps buffered SSE events per stream for Last-Event-ID
	// replay. Zero → 1024.
	EventBufferSize int

	// Authorize is an optional hook invoked before handling each request.
	// Return a non-nil error to reject with 401 Unauthorized.
	Authorize func(r *http.Request) error

	// PreferSSE makes POST request responses use text/event-stream even when
	// a single JSON object would suffice. Default false (application/json).
	PreferSSE bool

	// SSERetryMS is the retry field sent when a stream connection is closed
	// without terminating the logical stream. Zero → 3000.
	SSERetryMS int

	// HeartbeatInterval sends SSE comments on long-lived GET streams.
	// Zero → 15s. Negative disables heartbeats.
	HeartbeatInterval time.Duration
}

// streamableHandler implements Streamable HTTP for an MCP Server.
type streamableHandler struct {
	server  *Server
	opts    StreamableHTTPOptions
	sessions *sessionManager
}

// NewStreamableHTTPHandler returns an http.Handler for the Streamable HTTP transport
// (MCP spec 2025-11-25). Mount it on your mux at the MCP endpoint path.
func NewStreamableHTTPHandler(server *Server, opts StreamableHTTPOptions) http.Handler {
	if server == nil {
		panic("mcpruntime: NewStreamableHTTPHandler: server is nil")
	}
	h := &streamableHandler{
		server:   server,
		opts:     opts,
		sessions: newSessionManager(opts.SessionTTL),
	}
	if h.opts.SSERetryMS == 0 {
		h.opts.SSERetryMS = defaultSSERetryMS
	}
	if h.opts.HeartbeatInterval == 0 {
		h.opts.HeartbeatInterval = defaultHeartbeatInt
	}
	return h
}

// ServeStreamableHTTP listens on addr and serves Streamable HTTP until ctx is cancelled.
// Prefer binding to 127.0.0.1 for local servers to reduce DNS-rebinding risk.
func ServeStreamableHTTP(ctx context.Context, addr string, server *Server, opts StreamableHTTPOptions) error {
	path := opts.Path
	if path == "" {
		path = defaultHTTPPath
	}
	h := NewStreamableHTTPHandler(server, opts)
	mux := http.NewServeMux()
	mux.Handle(path, h)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		if sh, ok := h.(*streamableHandler); ok {
			sh.sessions.Close()
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if sh, ok := h.(*streamableHandler); ok {
			sh.sessions.Close()
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (h *streamableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateOrigin(r, h.opts.AllowedOrigins, h.opts.AllowAllOrigins) {
		h.writeHTTPError(w, http.StatusForbidden, "forbidden origin")
		return
	}
	if h.opts.Authorize != nil {
		if err := h.opts.Authorize(r); err != nil {
			h.writeHTTPError(w, http.StatusUnauthorized, err.Error())
			return
		}
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePOST(w, r)
	case http.MethodGet:
		h.handleGET(w, r)
	case http.MethodDelete:
		h.handleDELETE(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		h.writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *streamableHandler) handlePOST(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if !acceptIncludes(accept, contentTypeJSON) || !acceptIncludes(accept, contentTypeSSE) {
		h.writeHTTPError(w, http.StatusBadRequest, "Accept must include application/json and text/event-stream")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxHTTPBodyBytes+1))
	if err != nil {
		h.writeHTTPError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(body) > maxHTTPBodyBytes {
		h.writeHTTPError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	if len(body) == 0 {
		h.writeHTTPError(w, http.StatusBadRequest, "empty body")
		return
	}

	// Batch not supported over Streamable HTTP.
	trimmed := trimLeft(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		h.writeHTTPError(w, http.StatusBadRequest, "JSON-RPC batches are not supported over Streamable HTTP")
		return
	}

	msgKind, method, err := classifyJSONRPC(body)
	if err != nil {
		h.writeHTTPError(w, http.StatusBadRequest, "invalid JSON-RPC message")
		return
	}

	sessionID := r.Header.Get(headerMCPSessionID)
	var sess *Session

	if method == "initialize" && msgKind == rpcKindRequest {
		if sessionID != "" {
			// Re-initialize on existing session is invalid; require fresh session.
			sess = h.sessions.Get(sessionID)
			if sess == nil {
				h.writeHTTPError(w, http.StatusNotFound, "session not found")
				return
			}
		} else {
			sess = h.sessions.Create()
			if h.opts.EventBufferSize > 0 && sess.streams != nil {
				sess.streams.setBufferSize(h.opts.EventBufferSize)
			}
		}
	} else {
		if sessionID == "" {
			h.writeHTTPError(w, http.StatusBadRequest, "missing MCP-Session-Id")
			return
		}
		sess = h.sessions.Get(sessionID)
		if sess == nil {
			h.writeHTTPError(w, http.StatusNotFound, "session not found")
			return
		}
		if _, err := resolveProtocolVersion(r.Header.Get(headerMCPProtocolVersion), sess); err != nil {
			h.writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	ctx := WithSession(r.Context(), sess)

	switch msgKind {
	case rpcKindNotification, rpcKindResponse:
		// Handle notification/response; no JSON-RPC result body.
		_ = h.server.HandleRaw(ctx, body)
		w.WriteHeader(http.StatusAccepted)
		return

	case rpcKindRequest:
		useSSE := h.opts.PreferSSE
		resp := h.server.HandleRaw(ctx, body)
		// HandleRaw always returns a response for requests with id.
		if resp == nil {
			// Treat as accepted (edge case).
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// Set session id on initialize responses.
		if method == "initialize" {
			w.Header().Set(headerMCPSessionID, sess.ID)
		}

		if useSSE {
			h.writePOSTSSE(w, r, sess, resp)
			return
		}

		w.Header().Set("Content-Type", contentTypeJSON)
		if method == "initialize" {
			// Header already set.
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
		return

	default:
		h.writeHTTPError(w, http.StatusBadRequest, "unsupported JSON-RPC message kind")
	}
}

func (h *streamableHandler) writePOSTSSE(w http.ResponseWriter, _ *http.Request, sess *Session, resp []byte) {
	writeSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	flushWriter(w)

	st := sess.streams.openStream("post")
	defer sess.streams.closeStream(st.id)

	// Prime event (spec SHOULD).
	primeID, _ := st.publish(nil)
	_ = writeSSEEvent(w, primeID, nil)
	flushWriter(w)

	// Publish and write the JSON-RPC response.
	id, err := st.publish(resp)
	if err != nil {
		return
	}
	_ = writeSSEEvent(w, id, resp)
	flushWriter(w)
	// Stream terminates after response (spec SHOULD).
}

func (h *streamableHandler) handleGET(w http.ResponseWriter, r *http.Request) {
	if !acceptIncludes(r.Header.Get("Accept"), contentTypeSSE) {
		h.writeHTTPError(w, http.StatusBadRequest, "Accept must include text/event-stream")
		return
	}

	sessionID := r.Header.Get(headerMCPSessionID)
	if sessionID == "" {
		h.writeHTTPError(w, http.StatusBadRequest, "missing MCP-Session-Id")
		return
	}
	sess := h.sessions.Get(sessionID)
	if sess == nil {
		h.writeHTTPError(w, http.StatusNotFound, "session not found")
		return
	}
	if _, err := resolveProtocolVersion(r.Header.Get(headerMCPProtocolVersion), sess); err != nil {
		h.writeHTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !sess.IsReady() {
		h.writeHTTPError(w, http.StatusBadRequest, "session not initialized")
		return
	}

	lastEventID := r.Header.Get(headerLastEventID)
	var replay []sseEvent

	// Prefer resuming the stream named in Last-Event-ID; otherwise the
	// session-scoped listen stream (reused across GET reconnects).
	var st *messageStream
	if lastEventID != "" {
		streamID, ok := streamIDFromEventID(lastEventID)
		if !ok {
			h.writeHTTPError(w, http.StatusBadRequest, "invalid Last-Event-ID")
			return
		}
		st = sess.streams.getStream(streamID)
		if st == nil {
			// Buffer expired or stream was a short-lived POST stream: fall back
			// to the listen stream without replay.
			st = sess.streams.openStream("listen")
		} else {
			var err error
			replay, err = st.eventsAfter(lastEventID)
			if err != nil {
				// Different stream id encoded in Last-Event-ID than this stream.
				h.writeHTTPError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	} else {
		st = sess.streams.openStream("listen")
	}

	writeSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	flushWriter(w)

	// Replay buffered events for resumability.
	for _, ev := range replay {
		if err := writeSSEEvent(w, ev.ID, ev.Data); err != nil {
			return
		}
		flushWriter(w)
	}

	// Prime only on a fresh listen connection (no Last-Event-ID).
	if lastEventID == "" && len(replay) == 0 {
		// Only prime if the stream has no events yet.
		st.mu.Lock()
		empty := len(st.buffer) == 0
		st.mu.Unlock()
		if empty {
			primeID, _ := st.publish(nil)
			_ = writeSSEEvent(w, primeID, nil)
			flushWriter(w)
		}
	}

	sub := st.subscribe(32)
	defer st.unsubscribe(sub)
	// Keep stream buffer for Last-Event-ID; do not closeStream on GET disconnect.

	var heartbeat <-chan time.Time
	var ticker *time.Ticker
	if h.opts.HeartbeatInterval > 0 {
		ticker = time.NewTicker(h.opts.HeartbeatInterval)
		defer ticker.Stop()
		heartbeat = ticker.C
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, ev.ID, ev.Data); err != nil {
				return
			}
			flushWriter(w)
		case <-heartbeat:
			if err := writeSSEComment(w, "ping"); err != nil {
				return
			}
			flushWriter(w)
		}
	}
}

func (h *streamableHandler) handleDELETE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(headerMCPSessionID)
	if sessionID == "" {
		h.writeHTTPError(w, http.StatusBadRequest, "missing MCP-Session-Id")
		return
	}
	if !h.sessions.Delete(sessionID) {
		h.writeHTTPError(w, http.StatusNotFound, "session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *streamableHandler) writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	// Optional JSON-RPC error response without id (spec MAY).
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"error": map[string]any{
			"code":    CodeInvalidRequest,
			"message": message,
		},
		"id": nil,
	})
}

// rpcMessageKind classifies a JSON-RPC message.
type rpcMessageKind int

const (
	rpcKindUnknown rpcMessageKind = iota
	rpcKindRequest
	rpcKindNotification
	rpcKindResponse
)

func classifyJSONRPC(raw []byte) (rpcMessageKind, string, error) {
	var probe struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return rpcKindUnknown, "", err
	}
	if probe.Method != "" {
		// JSON-RPC 2.0: notifications omit id.
		if probe.ID == nil {
			return rpcKindNotification, probe.Method, nil
		}
		return rpcKindRequest, probe.Method, nil
	}
	if probe.Result != nil || probe.Error != nil {
		return rpcKindResponse, "", nil
	}
	return rpcKindUnknown, "", errors.New("unclassified JSON-RPC message")
}
