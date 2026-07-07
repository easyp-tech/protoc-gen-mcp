package mcpruntime

import (
	"context"
	"testing"
	"time"
)

// TestHandleResourcesReadNoDeadlock verifies that resources/read does not hold
// the server read lock while invoking the resource handler. A handler that
// registers another primitive (taking the write lock) must not deadlock.
func TestHandleResourcesReadNoDeadlock(t *testing.T) {
	server := NewServer("test-server", "v0.0.1")

	server.AddResource(&Resource{Name: "self", URI: "self://a"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		// Registering during the handler acquires the write lock; this would
		// deadlock if resources/read still held the read lock.
		server.AddResource(&Resource{Name: "added", URI: "self://b"}, nil)
		return &ReadResourceResult{Contents: []*ResourceContents{{URI: req.URI, Text: "ok"}}}, nil
	})

	server.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))

	done := make(chan struct{})
	go func() {
		server.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"self://a"}}`))
		close(done)
	}()

	select {
	case <-done:
		// Completed without deadlocking.
	case <-time.After(2 * time.Second):
		t.Fatal("resources/read deadlocked: handler that registers a resource never returned")
	}
}
