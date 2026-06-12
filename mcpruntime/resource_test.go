package mcpruntime

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestExtractURIParams_SimpleTemplate(t *testing.T) {
	params, err := ExtractURIParams("users://alice", "users://{id}")
	if err != nil {
		t.Fatalf("ExtractURIParams: %v", err)
	}
	if got := params["id"]; got != "alice" {
		t.Fatalf("params[id] = %q, want %q", got, "alice")
	}
	if len(params) != 1 {
		t.Fatalf("len(params) = %d, want 1", len(params))
	}
}

func TestExtractURIParams_MultipleParams(t *testing.T) {
	params, err := ExtractURIParams("org://acme/users/bob", "org://{org}/users/{user}")
	if err != nil {
		t.Fatalf("ExtractURIParams: %v", err)
	}
	if got := params["org"]; got != "acme" {
		t.Fatalf("params[org] = %q, want %q", got, "acme")
	}
	if got := params["user"]; got != "bob" {
		t.Fatalf("params[user] = %q, want %q", got, "bob")
	}
	if len(params) != 2 {
		t.Fatalf("len(params) = %d, want 2", len(params))
	}
}

func TestExtractURIParams_MismatchedURI(t *testing.T) {
	_, err := ExtractURIParams("projects://123", "users://{id}")
	if err == nil {
		t.Fatal("expected error for mismatched URI")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %q, want 'does not match'", err.Error())
	}
}

func TestExtractURIParams_EmptyParam(t *testing.T) {
	// With [^/]+ regex, an empty segment causes a mismatch, which is still
	// the correct error path — the URI is invalid for this template.
	_, err := ExtractURIParams("users:///trailing", "users://{id}/trailing")
	if err == nil {
		t.Fatal("expected error for empty param value")
	}
	// Accept either "does not match" or "empty value" — both indicate rejection.
	if !strings.Contains(err.Error(), "does not match") && !strings.Contains(err.Error(), "empty value") {
		t.Fatalf("error = %q, want 'does not match' or 'empty value'", err.Error())
	}
}

func TestMarshalResourceContent_ProtoJSON(t *testing.T) {
	// Use structpb.Struct for a regular JSON object (not well-known wrapper type
	// which has special ProtoJSON serialization).
	msg, err := structpb.NewStruct(map[string]interface{}{
		"version":     "1.0",
		"environment": "production",
	})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}

	contents, err := MarshalResourceContent("test://resource/1", "application/json", msg)
	if err != nil {
		t.Fatalf("MarshalResourceContent: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1", len(contents))
	}

	rc := contents[0]
	if rc.URI != "test://resource/1" {
		t.Fatalf("URI = %q, want %q", rc.URI, "test://resource/1")
	}
	if rc.MIMEType != "application/json" {
		t.Fatalf("MIMEType = %q, want %q", rc.MIMEType, "application/json")
	}
	if rc.Text == "" {
		t.Fatal("Text is empty, want ProtoJSON content")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(rc.Text), &decoded); err != nil {
		t.Fatalf("unmarshal Text as JSON: %v", err)
	}
	if got, ok := decoded["version"]; !ok || got != "1.0" {
		t.Fatalf("decoded[version] = %v, want %q", got, "1.0")
	}
}

func TestMarshalResourceContent_CustomMIMEType(t *testing.T) {
	msg, err := structpb.NewStruct(map[string]interface{}{
		"count": float64(42),
	})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}

	contents, err := MarshalResourceContent("items://42", "application/vnd.custom+json", msg)
	if err != nil {
		t.Fatalf("MarshalResourceContent: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1", len(contents))
	}

	rc := contents[0]
	if rc.MIMEType != "application/vnd.custom+json" {
		t.Fatalf("MIMEType = %q, want %q", rc.MIMEType, "application/vnd.custom+json")
	}
	if rc.URI != "items://42" {
		t.Fatalf("URI = %q, want %q", rc.URI, "items://42")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(rc.Text), &decoded); err != nil {
		t.Fatalf("unmarshal Text as JSON: %v", err)
	}
	if got := decoded["count"]; got != float64(42) {
		t.Fatalf("decoded[count] = %v, want 42", got)
	}
}
