package mcpruntime

import (
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestParsePromptArguments_StringField(t *testing.T) {
	msg := &wrapperspb.StringValue{}
	err := ParsePromptArguments(map[string]string{"value": "hello world"}, msg, []string{"value"})
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != "hello world" {
		t.Fatalf("Value = %q, want %q", msg.Value, "hello world")
	}
}

func TestParsePromptArguments_MissingRequired(t *testing.T) {
	msg := &wrapperspb.StringValue{}
	err := ParsePromptArguments(map[string]string{}, msg, []string{"value"})
	if err == nil {
		t.Fatal("expected error for missing required arg")
	}
	if got := err.Error(); got != `missing required argument "value"` {
		t.Fatalf("error = %q, want missing required", got)
	}
}

func TestParsePromptArguments_MissingOptional(t *testing.T) {
	msg := &wrapperspb.StringValue{}
	err := ParsePromptArguments(map[string]string{}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != "" {
		t.Fatalf("Value = %q, want empty (zero-value)", msg.Value)
	}
}

func TestParsePromptArguments_UnknownArg(t *testing.T) {
	msg := &wrapperspb.StringValue{}
	err := ParsePromptArguments(map[string]string{"unknown_field": "ignored"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v (should ignore unknown)", err)
	}
}

func TestParsePromptArguments_Int32Field(t *testing.T) {
	msg := &wrapperspb.Int32Value{}
	err := ParsePromptArguments(map[string]string{"value": "42"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != 42 {
		t.Fatalf("Value = %d, want 42", msg.Value)
	}
}

func TestParsePromptArguments_BoolField(t *testing.T) {
	msg := &wrapperspb.BoolValue{}
	err := ParsePromptArguments(map[string]string{"value": "true"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if !msg.Value {
		t.Fatal("Value = false, want true")
	}
}

func TestParsePromptArguments_Float(t *testing.T) {
	msg := &wrapperspb.FloatValue{}
	err := ParsePromptArguments(map[string]string{"value": "3.14"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value < 3.13 || msg.Value > 3.15 {
		t.Fatalf("Value = %f, want ~3.14", msg.Value)
	}
}

func TestParsePromptArguments_Double(t *testing.T) {
	msg := &wrapperspb.DoubleValue{}
	err := ParsePromptArguments(map[string]string{"value": "2.718281828"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value < 2.71 || msg.Value > 2.72 {
		t.Fatalf("Value = %f, want ~2.718", msg.Value)
	}
}

func TestParsePromptArguments_UInt32(t *testing.T) {
	msg := &wrapperspb.UInt32Value{}
	err := ParsePromptArguments(map[string]string{"value": "100"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != 100 {
		t.Fatalf("Value = %d, want 100", msg.Value)
	}
}

func TestParsePromptArguments_UInt64(t *testing.T) {
	msg := &wrapperspb.UInt64Value{}
	err := ParsePromptArguments(map[string]string{"value": "999"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != 999 {
		t.Fatalf("Value = %d, want 999", msg.Value)
	}
}

func TestParsePromptArguments_Int64(t *testing.T) {
	msg := &wrapperspb.Int64Value{}
	err := ParsePromptArguments(map[string]string{"value": "-12345"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if msg.Value != -12345 {
		t.Fatalf("Value = %d, want -12345", msg.Value)
	}
}

func TestParsePromptArguments_BytesField(t *testing.T) {
	msg := &wrapperspb.BytesValue{}
	encoded := base64.StdEncoding.EncodeToString([]byte("hello bytes"))
	err := ParsePromptArguments(map[string]string{"value": encoded}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments: %v", err)
	}
	if string(msg.Value) != "hello bytes" {
		t.Fatalf("Value = %q, want %q", string(msg.Value), "hello bytes")
	}
}

func TestParsePromptArguments_InvalidNumber(t *testing.T) {
	msg := &wrapperspb.Int32Value{}
	err := ParsePromptArguments(map[string]string{"value": "abc"}, msg, nil)
	if err == nil {
		t.Fatal("expected error for invalid int32")
	}
	if got := err.Error(); !contains(got, "invalid value") {
		t.Fatalf("error = %q, want 'invalid value'", got)
	}
}

func TestParsePromptArguments_EnumField(t *testing.T) {
	// Build a dynamic message descriptor with an enum field inline.
	msgDesc := buildDynamicEnumMessage(t)
	fd := msgDesc.Fields().ByName("level")
	if fd == nil {
		t.Fatal("field 'level' not found in dynamic descriptor")
	}

	// Test: parse by enum name.
	msg := dynamicpb.NewMessage(msgDesc)
	err := ParsePromptArguments(map[string]string{"level": "EXPERTISE_LEVEL_INTERMEDIATE"}, msg, nil)
	if err != nil {
		t.Fatalf("ParsePromptArguments with enum name: %v", err)
	}
	got := msg.Get(fd).Enum()
	if got != 2 {
		t.Fatalf("enum Value = %d, want 2 (EXPERTISE_LEVEL_INTERMEDIATE)", got)
	}

	// Test: invalid enum value string returns error.
	msg2 := dynamicpb.NewMessage(msgDesc)
	err = ParsePromptArguments(map[string]string{"level": "INVALID_VALUE"}, msg2, nil)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
}

// buildDynamicEnumMessage builds a file/message/enum descriptor inline
// so we can test enum parsing without importing generated testproto
// (which would cause a circular import with mcpruntime).
func buildDynamicEnumMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()

	syntax := "proto3"
	pkg := "test.v1"
	enumName := "ExpertiseLevel"
	msgName := "TestEnumMsg"
	fieldName := "level"
	fileName := "test/v1/test.proto"
	var fieldNum int32 = 1

	v0 := int32(0)
	v1 := int32(1)
	v2 := int32(2)

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    &fileName,
		Package: &pkg,
		Syntax:  &syntax,
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: &enumName,
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: strPtr("EXPERTISE_LEVEL_NONE"), Number: &v0},
					{Name: strPtr("EXPERTISE_LEVEL_BEGINNER"), Number: &v1},
					{Name: strPtr("EXPERTISE_LEVEL_INTERMEDIATE"), Number: &v2},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: &msgName,
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     &fieldName,
						Number:   &fieldNum,
						Type:     enumTypePtr(descriptorpb.FieldDescriptorProto_TYPE_ENUM),
						TypeName: strPtr(".test.v1.ExpertiseLevel"),
					},
				},
			},
		},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd.Messages().ByName(protoreflect.Name(msgName))
}

func strPtr(s string) *string { return &s }

func enumTypePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
