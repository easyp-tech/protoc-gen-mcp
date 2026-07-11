package mcpruntime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const defaultEventBufferSize = 1024

// sseEvent is one buffered SSE message for resumability.
type sseEvent struct {
	ID   string
	Data []byte
}

// messageStream is one SSE stream within a session (GET listen or POST request-scoped).
type messageStream struct {
	id     string
	mu     sync.Mutex
	seq    int64
	buffer []sseEvent
	maxBuf int
	// live subscribers receive events as they are published.
	subs map[chan sseEvent]struct{}
	closed bool
}

func newMessageStream(id string, maxBuf int) *messageStream {
	if maxBuf <= 0 {
		maxBuf = defaultEventBufferSize
	}
	return &messageStream{
		id:     id,
		maxBuf: maxBuf,
		subs:   make(map[chan sseEvent]struct{}),
	}
}

func (st *messageStream) nextEventID() string {
	st.seq++
	return fmt.Sprintf("%s_%d", st.id, st.seq)
}

// publish stores an event and fans out to live subscribers. Returns the event ID.
func (st *messageStream) publish(data []byte) (string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.closed {
		return "", fmt.Errorf("stream closed")
	}
	id := st.nextEventID()
	ev := sseEvent{ID: id, Data: append([]byte(nil), data...)}
	st.buffer = append(st.buffer, ev)
	if len(st.buffer) > st.maxBuf {
		st.buffer = st.buffer[len(st.buffer)-st.maxBuf:]
	}
	for ch := range st.subs {
		select {
		case ch <- ev:
		default:
			// Slow consumer: drop this fan-out; buffer still holds the event for resume.
		}
	}
	return id, nil
}

func (st *messageStream) subscribe(buf int) chan sseEvent {
	if buf <= 0 {
		buf = 16
	}
	ch := make(chan sseEvent, buf)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.closed {
		close(ch)
		return ch
	}
	st.subs[ch] = struct{}{}
	return ch
}

func (st *messageStream) unsubscribe(ch chan sseEvent) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.subs[ch]; ok {
		delete(st.subs, ch)
		close(ch)
	}
}

// eventsAfter returns buffered events with IDs after lastEventID on this stream.
// lastEventID format: {streamID}_{seq}. If lastEventID is empty, returns nothing
// (live subscription continues). If lastEventID references another stream, returns error.
func (st *messageStream) eventsAfter(lastEventID string) ([]sseEvent, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if lastEventID == "" {
		return nil, nil
	}
	streamID, seq, ok := parseEventID(lastEventID)
	if !ok {
		return nil, fmt.Errorf("invalid Last-Event-ID")
	}
	if streamID != st.id {
		return nil, fmt.Errorf("Last-Event-ID refers to a different stream")
	}

	var out []sseEvent
	for _, ev := range st.buffer {
		_, evSeq, ok := parseEventID(ev.ID)
		if !ok {
			continue
		}
		if evSeq > seq {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (st *messageStream) close() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.closed {
		return
	}
	st.closed = true
	for ch := range st.subs {
		close(ch)
		delete(st.subs, ch)
	}
}

// sessionStreams manages SSE streams for one session.
type sessionStreams struct {
	mu      sync.Mutex
	maxBuf  int
	streams map[string]*messageStream
	// listenOrder preserves GET stream preference for notifications (first open wins per message).
	listenIDs []string
	nextID    int64
}

func newSessionStreams(maxBuf int) *sessionStreams {
	if maxBuf <= 0 {
		maxBuf = defaultEventBufferSize
	}
	return &sessionStreams{
		maxBuf:  maxBuf,
		streams: make(map[string]*messageStream),
	}
}

func (ss *sessionStreams) setBufferSize(n int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if n > 0 {
		ss.maxBuf = n
	}
}

func (ss *sessionStreams) openStream(kind string) *messageStream {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Reuse a single listen stream per session so GET reconnects with
	// Last-Event-ID can find the same buffer after the previous connection ends.
	if kind == "listen" {
		for _, id := range ss.listenIDs {
			if st, ok := ss.streams[id]; ok {
				// Re-open for new subscribers if previously closed for fan-out.
				st.mu.Lock()
				st.closed = false
				if st.subs == nil {
					st.subs = make(map[chan sseEvent]struct{})
				}
				st.mu.Unlock()
				return st
			}
		}
	}

	ss.nextID++
	id := fmt.Sprintf("%s%d", kind, ss.nextID)
	st := newMessageStream(id, ss.maxBuf)
	ss.streams[id] = st
	if kind == "listen" {
		ss.listenIDs = append(ss.listenIDs, id)
	}
	return st
}

func (ss *sessionStreams) getStream(id string) *messageStream {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.streams[id]
}

func (ss *sessionStreams) closeStream(id string) {
	ss.mu.Lock()
	st := ss.streams[id]
	if st != nil {
		delete(ss.streams, id)
		// remove from listenIDs
		for i, lid := range ss.listenIDs {
			if lid == id {
				ss.listenIDs = append(ss.listenIDs[:i], ss.listenIDs[i+1:]...)
				break
			}
		}
	}
	ss.mu.Unlock()
	if st != nil {
		st.close()
	}
}

func (ss *sessionStreams) closeAll() {
	ss.mu.Lock()
	streams := make([]*messageStream, 0, len(ss.streams))
	for id, st := range ss.streams {
		streams = append(streams, st)
		delete(ss.streams, id)
	}
	ss.listenIDs = nil
	ss.mu.Unlock()
	for _, st := range streams {
		st.close()
	}
}

// broadcastNotification sends to exactly one listen stream (first open), per MCP
// "MUST NOT broadcast the same message across multiple streams".
func (ss *sessionStreams) broadcastNotification(method string, params any) error {
	payload, err := marshalJSONRPCNotification(method, params)
	if err != nil {
		return err
	}

	ss.mu.Lock()
	var target *messageStream
	for _, id := range ss.listenIDs {
		if st, ok := ss.streams[id]; ok {
			target = st
			break
		}
	}
	ss.mu.Unlock()

	if target == nil {
		return nil
	}
	_, err = target.publish(payload)
	return err
}

func marshalJSONRPCNotification(method string, params any) ([]byte, error) {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	return json.Marshal(msg)
}

// parseEventID splits "{streamID}_{seq}" into parts.
func parseEventID(id string) (streamID string, seq int64, ok bool) {
	// stream IDs are like "listen1" or "post2" (no underscore in stream id).
	// Event IDs are "{streamID}_{seq}".
	i := strings.LastIndex(id, "_")
	if i <= 0 || i == len(id)-1 {
		return "", 0, false
	}
	streamID = id[:i]
	n, err := strconv.ParseInt(id[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return streamID, n, true
}

// streamIDFromEventID extracts stream id from an event id.
func streamIDFromEventID(eventID string) (string, bool) {
	streamID, _, ok := parseEventID(eventID)
	return streamID, ok
}
