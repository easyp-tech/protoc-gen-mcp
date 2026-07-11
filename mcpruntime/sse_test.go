package mcpruntime

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteSSEEvent(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSSEEvent(&buf, "listen1_1", []byte(`{"jsonrpc":"2.0"}`)); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "id: listen1_1\n") {
		t.Fatalf("missing id: %q", got)
	}
	if !strings.Contains(got, "data: {\"jsonrpc\":\"2.0\"}\n") {
		t.Fatalf("missing data: %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("expected trailing blank line: %q", got)
	}
}

func TestWriteSSEEvent_EmptyPrime(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSSEEvent(&buf, "listen1_1", nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got != "id: listen1_1\ndata: \n\n" {
		t.Fatalf("prime event = %q", got)
	}
}

func TestParseEventID(t *testing.T) {
	sid, seq, ok := parseEventID("listen1_42")
	if !ok || sid != "listen1" || seq != 42 {
		t.Fatalf("got %s %d %v", sid, seq, ok)
	}
	if _, _, ok := parseEventID("bad"); ok {
		t.Fatal("expected failure")
	}
}
