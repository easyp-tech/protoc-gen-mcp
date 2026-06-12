package codegen

import (
	"strings"
	"testing"
)

func TestCollectPrompts_RecognizesPromptMessage(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message ReviewCode {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "review_code"`,
			`    description: "Review source code"`,
			`    icons: [{ src: "https://example.com/icon.svg" }]`,
			`  };`,
			`  string code = 1;`,
			`}`,
			`message Summarize {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "summarize"`,
			`    title: "Document Summarizer"`,
			`    description: "Summarize text"`,
			`  };`,
			`  string text = 1;`,
			`}`,
			`message NotAPrompt {`,
			`  string data = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]
	if file == nil {
		t.Fatal("prompts proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Prompts) != 2 {
		t.Fatalf("prompt count = %d, want 2", len(model.Prompts))
	}

	p0 := model.Prompts[0]
	if p0.Name != "review_code" {
		t.Fatalf("Prompt[0].Name = %q, want %q", p0.Name, "review_code")
	}
	if p0.Description != "Review source code" {
		t.Fatalf("Prompt[0].Description = %q, want %q", p0.Description, "Review source code")
	}
	if p0.ProtoName != "ReviewCode" {
		t.Fatalf("Prompt[0].ProtoName = %q, want %q", p0.ProtoName, "ReviewCode")
	}
	if len(p0.Icons) != 1 {
		t.Fatalf("Prompt[0].Icons count = %d, want 1", len(p0.Icons))
	}

	p1 := model.Prompts[1]
	if p1.Name != "summarize" {
		t.Fatalf("Prompt[1].Name = %q, want %q", p1.Name, "summarize")
	}
	if p1.Title != "Document Summarizer" {
		t.Fatalf("Prompt[1].Title = %q, want %q", p1.Title, "Document Summarizer")
	}
}

func TestCollectPrompts_ArgumentRequiredness(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message TestPrompt {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "test"`,
			`    description: "Test prompt"`,
			`  };`,
			`  string required_field = 1;`,
			`  optional string optional_field = 2;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]
	if file == nil {
		t.Fatal("prompts proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Prompts) != 1 {
		t.Fatalf("prompt count = %d, want 1", len(model.Prompts))
	}

	args := model.Prompts[0].Arguments
	if len(args) != 2 {
		t.Fatalf("argument count = %d, want 2", len(args))
	}

	if !args[0].Required {
		t.Fatal("singular field should be required")
	}
	if args[1].Required {
		t.Fatal("optional field should not be required")
	}
}

func TestCollectPrompts_AcceptsScalarAndEnumTypes(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`enum Level { LEVEL_NONE = 0; LEVEL_HIGH = 1; }`,
			`message AllScalars {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "all_scalars"`,
			`    description: "All scalar types"`,
			`  };`,
			`  string s = 1;`,
			`  int32 i32 = 2;`,
			`  int64 i64 = 3;`,
			`  uint32 u32 = 4;`,
			`  uint64 u64 = 5;`,
			`  float f = 6;`,
			`  double d = 7;`,
			`  bool b = 8;`,
			`  bytes by = 9;`,
			`  Level level = 10;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]
	if file == nil {
		t.Fatal("prompts proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Prompts) != 1 {
		t.Fatalf("prompt count = %d, want 1", len(model.Prompts))
	}
	if len(model.Prompts[0].Arguments) != 10 {
		t.Fatalf("argument count = %d, want 10", len(model.Prompts[0].Arguments))
	}
}

func TestCollectPrompts_RejectsMessageField(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Inner { string x = 1; }`,
			`message Bad {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "bad"`,
			`    description: "Bad prompt"`,
			`  };`,
			`  Inner nested = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	_, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err == nil {
		t.Fatal("CollectFileModel unexpectedly succeeded for message field in prompt")
	}
	if !strings.Contains(err.Error(), "unsupported type message") {
		t.Fatalf("error = %v, want message type rejection", err)
	}
}

func TestCollectPrompts_RejectsRepeatedField(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Bad {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "bad"`,
			`    description: "Bad prompt"`,
			`  };`,
			`  repeated string tags = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	_, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err == nil {
		t.Fatal("CollectFileModel unexpectedly succeeded for repeated field in prompt")
	}
	if !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("error = %v, want repeated rejection", err)
	}
}

func TestCollectPrompts_RejectsMapField(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Bad {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "bad"`,
			`    description: "Bad prompt"`,
			`  };`,
			`  map<string, string> labels = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	_, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err == nil {
		t.Fatal("CollectFileModel unexpectedly succeeded for map field in prompt")
	}
	if !strings.Contains(err.Error(), "map") {
		t.Fatalf("error = %v, want map rejection", err)
	}
}

func TestCollectPrompts_RejectsOneofField(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Bad {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "bad"`,
			`    description: "Bad prompt"`,
			`  };`,
			`  oneof selector {`,
			`    string email = 1;`,
			`    string phone = 2;`,
			`  }`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	_, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err == nil {
		t.Fatal("CollectFileModel unexpectedly succeeded for oneof field in prompt")
	}
	if !strings.Contains(err.Error(), "oneof") {
		t.Fatalf("error = %v, want oneof rejection", err)
	}
}

func TestCollectPrompts_DefaultNameSnakeCase(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message MyLongPromptName {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    description: "No explicit name"`,
			`  };`,
			`  string text = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Prompts) != 1 {
		t.Fatalf("prompt count = %d, want 1", len(model.Prompts))
	}

	if got := model.Prompts[0].Name; got != "my_long_prompt_name" {
		t.Fatalf("Prompt.Name = %q, want %q", got, "my_long_prompt_name")
	}
}

func TestCollectPrompts_FieldDescriptionFromOptions(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/prompts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/prompts;promptsv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message FieldDesc {`,
			`  option (mcp.options.v1.prompt) = {`,
			`    name: "field_desc"`,
			`    description: "Test field description"`,
			`  };`,
			`  string code = 1 [(mcp.options.v1.field) = {`,
			`    description: "Source code to review"`,
			`  }];`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/prompts.proto")
	file := plugin.FilesByPath["test/v1/prompts.proto"]

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Prompts) != 1 || len(model.Prompts[0].Arguments) != 1 {
		t.Fatalf("unexpected prompt/argument count")
	}

	if got := model.Prompts[0].Arguments[0].Description; got != "Source code to review" {
		t.Fatalf("Argument.Description = %q, want %q", got, "Source code to review")
	}
}
