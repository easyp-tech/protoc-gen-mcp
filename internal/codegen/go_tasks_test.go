package codegen

import (
	"strings"
	"testing"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/compiler/protogen"
)

func goTaskSupportPlugin(t *testing.T, methodOptions string) *protogen.Plugin {
	t.Helper()
	return newTempProtogenPlugin(t, map[string]string{
		"test/v1/tasks.proto": `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";
import "mcp/options/v1/options.proto";
message Request {}
message Response {}
service TaskAPI {
  rpc Run(Request) returns (Response) {
    option (mcp.options.v1.method) = { ` + methodOptions + ` };
  }
}`,
	}, "test/v1/tasks.proto")
}

func TestGoTaskSupportRejectsUnsupportedDeclarations(t *testing.T) {
	for _, mode := range []string{"TASK_SUPPORT_OPTIONAL", "TASK_SUPPORT_REQUIRED", "99", "-1"} {
		t.Run(mode, func(t *testing.T) {
			plugin := goTaskSupportPlugin(t, "execution: { task_support: "+mode+" }")
			err := Generate(plugin, Options{Language: LanguageGo})
			if err == nil {
				t.Fatal("Go generation silently accepted unsupported task execution")
			}
			for _, detail := range []string{"test.v1.TaskAPI.Run", "test/v1/tasks.proto", "execution.task_support=" + mode, "lang=go", "2025-11-25", "TASK_SUPPORT_NONE"} {
				if !strings.Contains(err.Error(), detail) {
					t.Errorf("error %q does not identify %q", err, detail)
				}
			}
			if files := plugin.Response().GetFile(); len(files) != 0 {
				t.Fatalf("unsupported declaration emitted %d output files", len(files))
			}
		})
	}
}

func TestGoTaskSupportAllowsSynchronousDeclarations(t *testing.T) {
	for _, declaration := range []string{"", "execution: {}", "execution: { task_support: TASK_SUPPORT_NONE }"} {
		t.Run(declaration, func(t *testing.T) {
			plugin := goTaskSupportPlugin(t, declaration)
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatal(err)
			}
			source := string(generatedFileContent(t, plugin, "test/v1/tasks.mcp.go"))
			if !strings.Contains(source, "RegisterSDKProtoTool") {
				t.Fatal("synchronous protobuf tool was not generated")
			}
			if strings.Contains(source, "TaskSupport:") || strings.Contains(source, "taskSupport") {
				t.Fatal("synchronous binding advertises unsupported Tasks metadata")
			}
		})
	}
}

func TestGoTaskSupportIgnoresHiddenDeclarations(t *testing.T) {
	for _, mode := range []string{"TASK_SUPPORT_OPTIONAL", "TASK_SUPPORT_REQUIRED"} {
		t.Run(mode, func(t *testing.T) {
			plugin := goTaskSupportPlugin(t, "hidden: true execution: { task_support: "+mode+" }")
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatalf("hidden, unregistered tool must not block Go generation: %v", err)
			}
			if files := plugin.Response().GetFile(); len(files) != 0 {
				t.Fatalf("hidden-only service emitted %d files", len(files))
			}
		})
	}
}

func TestGoTaskSupportValidationDoesNotChangeOtherTargets(t *testing.T) {
	for _, language := range []Language{LanguagePython, LanguageKotlin, LanguageJava, LanguageTypeScript} {
		for _, mode := range []mcpoptionsv1.TaskSupport{mcpoptionsv1.TaskSupport_TASK_SUPPORT_OPTIONAL, mcpoptionsv1.TaskSupport_TASK_SUPPORT_REQUIRED} {
			t.Run(string(language)+"/"+mode.String(), func(t *testing.T) {
				plugin := goTaskSupportPlugin(t, "execution: { task_support: "+mode.String()+" }")
				model, err := CollectFileModel(plugin.FilesByPath["test/v1/tasks.proto"], Options{Language: language})
				if err != nil {
					t.Fatal(err)
				}
				if got := model.Services[0].Methods[0].TaskSupport; got != mode {
					t.Fatalf("task support = %v, want %v", got, mode)
				}
			})
		}
	}
}
