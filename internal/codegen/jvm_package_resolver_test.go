package codegen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
)

func TestResolveJVMFilePackage_UsesJavaPackageAndProtoFallback(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/java_package.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/javapackage;javapackagev1";`,
			`option java_package = "com.example.test";`,
			`option java_multiple_files = true;`,
			`message JavaPackageRequest {}`,
			"",
		}, "\n"),
		"test/v1/proto_fallback.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package fallback.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/protofallback;protofallbackv1";`,
			`message ProtoFallbackRequest {}`,
			"",
		}, "\n"),
	}, "test/v1/java_package.proto", "test/v1/proto_fallback.proto")

	javaPackage, err := resolveJVMFilePackage(plugin.FilesByPath["test/v1/java_package.proto"])
	if err != nil {
		t.Fatalf("resolve java package: %v", err)
	}
	if got, want := javaPackage.Package, "com.example.test"; got != want {
		t.Fatalf("java package Package = %q, want %q", got, want)
	}
	if !javaPackage.MultipleFiles {
		t.Fatal("java package MultipleFiles should be true")
	}

	fallback, err := resolveJVMFilePackage(plugin.FilesByPath["test/v1/proto_fallback.proto"])
	if err != nil {
		t.Fatalf("resolve proto fallback: %v", err)
	}
	if got, want := fallback.Package, "fallback.v1"; got != want {
		t.Fatalf("fallback Package = %q, want %q", got, want)
	}
	if got, want := fallback.OuterClassName, "ProtoFallbackOuterClass"; got != want {
		t.Fatalf("fallback OuterClassName = %q, want %q", got, want)
	}
}

func TestResolveJVMMessageTypeRef_RespectsJavaMultipleFilesAndOuterClassname(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`option java_package = "com.example.shared";`,
			`option java_multiple_files = true;`,
			`message SharedRequest {}`,
			"",
		}, "\n"),
		"test/v1/wrapped.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/wrapped;wrappedv1";`,
			`option java_package = "com.example.wrapped";`,
			`option java_multiple_files = false;`,
			`option java_outer_classname = "WrappedTypes";`,
			`message WrappedRequest {}`,
			"",
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`option java_package = "com.example.service";`,
			`option java_multiple_files = true;`,
			`import "test/v1/shared.proto";`,
			`import "test/v1/wrapped.proto";`,
			`message LocalRequest {}`,
			`service ResolverService {`,
			`  rpc UseShared(SharedRequest) returns (WrappedRequest);`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/service.proto")

	currentPath := "test/v1/service.proto"
	local, err := resolveJVMMessageTypeRef(findMessage(t, plugin.FilesByPath[currentPath], "LocalRequest"), currentPath)
	if err != nil {
		t.Fatalf("resolve local message: %v", err)
	}
	if got, want := local.Expr, "LocalRequest"; got != want {
		t.Fatalf("local Expr = %q, want %q", got, want)
	}
	if local.ImportPath != "" {
		t.Fatalf("local ImportPath = %q, want empty", local.ImportPath)
	}
	if !local.IsCurrentFile {
		t.Fatal("local IsCurrentFile should be true")
	}

	shared, err := resolveJVMMessageTypeRef(findMessage(t, plugin.FilesByPath["test/v1/shared.proto"], "SharedRequest"), currentPath)
	if err != nil {
		t.Fatalf("resolve shared message: %v", err)
	}
	if got, want := shared.ImportPath, "com.example.shared.SharedRequest"; got != want {
		t.Fatalf("shared ImportPath = %q, want %q", got, want)
	}
	if got, want := shared.Expr, "SharedRequest"; got != want {
		t.Fatalf("shared Expr = %q, want %q", got, want)
	}

	wrapped, err := resolveJVMMessageTypeRef(findMessage(t, plugin.FilesByPath["test/v1/wrapped.proto"], "WrappedRequest"), currentPath)
	if err != nil {
		t.Fatalf("resolve wrapped message: %v", err)
	}
	if got, want := wrapped.ImportPath, "com.example.wrapped.WrappedTypes"; got != want {
		t.Fatalf("wrapped ImportPath = %q, want %q", got, want)
	}
	if got, want := wrapped.Expr, "WrappedTypes.WrappedRequest"; got != want {
		t.Fatalf("wrapped Expr = %q, want %q", got, want)
	}
}

func TestResolveJVMMessageTypeRef_UsesDefaultOuterClassname(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/default_outer.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/defaultouter;defaultouterv1";`,
			`option java_package = "com.example.defaultouter";`,
			`option java_multiple_files = false;`,
			`message DefaultRequest {}`,
			"",
		}, "\n"),
	}, "test/v1/default_outer.proto")

	ref, err := resolveJVMMessageTypeRef(findMessage(t, plugin.FilesByPath["test/v1/default_outer.proto"], "DefaultRequest"), "other.proto")
	if err != nil {
		t.Fatalf("resolve default outer message: %v", err)
	}
	if got, want := ref.ImportPath, "com.example.defaultouter.DefaultOuterOuterClass"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := ref.Expr, "DefaultOuterOuterClass.DefaultRequest"; got != want {
		t.Fatalf("Expr = %q, want %q", got, want)
	}
}

func TestResolveJVMMessageTypeRef_CurrentFileSkipsImport(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/current_wrapper.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/current;currentv1";`,
			`option java_package = "com.example.current";`,
			`option java_multiple_files = false;`,
			`option java_outer_classname = "CurrentWrapper";`,
			`message CurrentRequest {}`,
			"",
		}, "\n"),
	}, "test/v1/current_wrapper.proto")

	ref, err := resolveJVMMessageTypeRef(findMessage(t, plugin.FilesByPath["test/v1/current_wrapper.proto"], "CurrentRequest"), "test/v1/current_wrapper.proto")
	if err != nil {
		t.Fatalf("resolve current wrapper message: %v", err)
	}
	if ref.ImportPath != "" {
		t.Fatalf("ImportPath = %q, want empty", ref.ImportPath)
	}
	if got, want := ref.Expr, "CurrentWrapper.CurrentRequest"; got != want {
		t.Fatalf("Expr = %q, want %q", got, want)
	}
}

func TestResolveJVMEnumTypeRef_RespectsJavaMultipleFilesAndOuterClassname(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/enums.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/enums;enumsv1";`,
			`option java_package = "com.example.enums";`,
			`option java_multiple_files = true;`,
			`enum SharedStatus { SHARED_STATUS_UNSPECIFIED = 0; }`,
			"",
		}, "\n"),
		"test/v1/wrapped_enums.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/wrappedenums;wrappedenumsv1";`,
			`option java_package = "com.example.enums.wrapped";`,
			`option java_multiple_files = false;`,
			`option java_outer_classname = "EnumWrapper";`,
			`enum WrappedStatus { WRAPPED_STATUS_UNSPECIFIED = 0; }`,
			"",
		}, "\n"),
		"test/v1/local_enums.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/localenums;localenumsv1";`,
			`option java_package = "com.example.local";`,
			`option java_multiple_files = true;`,
			`enum LocalStatus { LOCAL_STATUS_UNSPECIFIED = 0; }`,
			"",
		}, "\n"),
	}, "test/v1/local_enums.proto", "test/v1/enums.proto", "test/v1/wrapped_enums.proto")

	currentPath := "test/v1/local_enums.proto"
	local, err := resolveJVMEnumTypeRef(findEnum(t, plugin.FilesByPath[currentPath], "LocalStatus"), currentPath)
	if err != nil {
		t.Fatalf("resolve local enum: %v", err)
	}
	if local.ImportPath != "" {
		t.Fatalf("local ImportPath = %q, want empty", local.ImportPath)
	}
	if got, want := local.Expr, "LocalStatus"; got != want {
		t.Fatalf("local Expr = %q, want %q", got, want)
	}

	shared, err := resolveJVMEnumTypeRef(findEnum(t, plugin.FilesByPath["test/v1/enums.proto"], "SharedStatus"), currentPath)
	if err != nil {
		t.Fatalf("resolve shared enum: %v", err)
	}
	if got, want := shared.ImportPath, "com.example.enums.SharedStatus"; got != want {
		t.Fatalf("shared ImportPath = %q, want %q", got, want)
	}
	if got, want := shared.Expr, "SharedStatus"; got != want {
		t.Fatalf("shared Expr = %q, want %q", got, want)
	}

	wrapped, err := resolveJVMEnumTypeRef(findEnum(t, plugin.FilesByPath["test/v1/wrapped_enums.proto"], "WrappedStatus"), currentPath)
	if err != nil {
		t.Fatalf("resolve wrapped enum: %v", err)
	}
	if got, want := wrapped.ImportPath, "com.example.enums.wrapped.EnumWrapper"; got != want {
		t.Fatalf("wrapped ImportPath = %q, want %q", got, want)
	}
	if got, want := wrapped.Expr, "EnumWrapper.WrappedStatus"; got != want {
		t.Fatalf("wrapped Expr = %q, want %q", got, want)
	}
}

func findMessage(t *testing.T, file *protogen.File, name string) *protogen.Message {
	t.Helper()

	for _, message := range file.Messages {
		if string(message.Desc.Name()) == name {
			return message
		}
	}

	t.Fatalf("message %q not found in %s", name, file.Desc.Path())
	return nil
}

func findEnum(t *testing.T, file *protogen.File, name string) *protogen.Enum {
	t.Helper()

	for _, enum := range file.Enums {
		if string(enum.Desc.Name()) == name {
			return enum
		}
	}

	t.Fatalf("enum %q not found in %s", name, file.Desc.Path())
	return nil
}
