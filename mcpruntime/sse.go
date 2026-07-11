package mcpruntime

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	contentTypeJSON = "application/json"
	contentTypeSSE  = "text/event-stream"
)

// writeSSEHeaders sets standard SSE response headers (does not write status).
func writeSSEHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", contentTypeSSE)
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Disable proxy buffering where supported.
	h.Set("X-Accel-Buffering", "no")
}

// writeSSEEvent writes one SSE event. data must not contain raw newlines unescaped;
// JSON-RPC messages are single-line. Empty data is allowed (prime event).
func writeSSEEvent(w io.Writer, id string, data []byte) error {
	var b strings.Builder
	if id != "" {
		b.WriteString("id: ")
		b.WriteString(id)
		b.WriteByte('\n')
	}
	if len(data) == 0 {
		b.WriteString("data: \n\n")
	} else {
		// Split on newlines for safety (JSON-RPC should be one line).
		for _, line := range strings.Split(string(data), "\n") {
			b.WriteString("data: ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeSSERetry writes a retry field (milliseconds) before optional close.
func writeSSERetry(w io.Writer, ms int) error {
	if ms <= 0 {
		return nil
	}
	_, err := fmt.Fprintf(w, "retry: %d\n\n", ms)
	return err
}

// writeSSEComment writes an SSE comment (e.g. keepalive ping).
func writeSSEComment(w io.Writer, comment string) error {
	_, err := fmt.Fprintf(w, ": %s\n\n", comment)
	return err
}

// flushWriter flushes if the writer supports it.
func flushWriter(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// acceptIncludes reports whether Accept header lists mediaType (substring match
// per common MCP client practice, including */*).
func acceptIncludes(accept, mediaType string) bool {
	accept = strings.ToLower(accept)
	mediaType = strings.ToLower(mediaType)
	if accept == "" {
		return false
	}
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, ';'); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		if part == "*/*" || part == mediaType {
			return true
		}
		// type/*
		if strings.HasSuffix(part, "/*") {
			prefix := strings.TrimSuffix(part, "/*")
			if strings.HasPrefix(mediaType, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// parseRetryMS is a small helper for tests.
func parseRetryMS(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "retry:") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "retry:")))
	return n, err == nil
}
