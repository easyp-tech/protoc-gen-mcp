package codegen

import (
	"reflect"
	"strings"
	"testing"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
)

func TestTypeScriptModel_StructuralIRDoesNotContainRawDescriptorsOrSDKTypes(t *testing.T) {
	disallowed := map[string]struct{}{
		"google.golang.org/protobuf/compiler/protogen.Enum":              {},
		"google.golang.org/protobuf/compiler/protogen.Field":             {},
		"google.golang.org/protobuf/compiler/protogen.File":              {},
		"google.golang.org/protobuf/compiler/protogen.Message":           {},
		"google.golang.org/protobuf/compiler/protogen.Method":            {},
		"google.golang.org/protobuf/compiler/protogen.Service":           {},
		"google.golang.org/protobuf/reflect/protoreflect.Descriptor":     {},
		"google.golang.org/protobuf/reflect/protoreflect.FileDescriptor": {},
	}

	assertIRTypeHasNoDisallowedFields(t, reflect.TypeOf(TypeScriptFileModel{}), disallowed)
	assertIRTypeHasNoPackageSubstring(t, reflect.TypeOf(TypeScriptFileModel{}), "modelcontextprotocol")
	assertIRTypeHasNoPackageSubstring(t, reflect.TypeOf(TypeScriptFileModel{}), "bufbuild/protobuf")
}

func TestTypeScriptModel_PreservesSharedToolSemantics(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/metadata.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/metadata;metadatav1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Request {}`,
			`message Response {}`,
			`service MetadataService {`,
			`  option (mcp.options.v1.service) = {`,
			`    namespace: "metadata"`,
			`    icons: [{`,
			`      src: "https://example.com/service.svg"`,
			`      mime_type: "image/svg+xml"`,
			`      sizes: "64x64"`,
			`      theme: "light"`,
			`    }]`,
			`  };`,
			``,
			`  rpc Visible(Request) returns (Response) {`,
			`    option (mcp.options.v1.method) = {`,
			`      name: "metadata_visible"`,
			`      title: "Visible metadata"`,
			`      description: "Returns metadata."`,
			`      annotations: { read_only_hint: true open_world_hint: false }`,
			`      execution: { task_support: TASK_SUPPORT_OPTIONAL }`,
			`    };`,
			`  }`,
			``,
			`  rpc OverrideIcon(Request) returns (Response) {`,
			`    option (mcp.options.v1.method) = {`,
			`      icons: [{`,
			`        src: "https://example.com/method.png"`,
			`        mime_type: "image/png"`,
			`        sizes: "32x32"`,
			`        theme: "dark"`,
			`      }]`,
			`      execution: { task_support: TASK_SUPPORT_REQUIRED }`,
			`    };`,
			`  }`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/metadata.proto")
	file := plugin.FilesByPath["test/v1/metadata.proto"]
	if file == nil {
		t.Fatal("metadata proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageTypeScript})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	tsModel, err := CollectTypeScriptFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectTypeScriptFileModel: %v", err)
	}

	if got, want := tsModel.ProtoPath, shared.ProtoPath; got != want {
		t.Fatalf("ProtoPath = %q, want %q", got, want)
	}
	if got, want := tsModel.GeneratedFilenamePrefix, shared.GeneratedFilenamePrefix; got != want {
		t.Fatalf("GeneratedFilenamePrefix = %q, want %q", got, want)
	}
	if len(tsModel.Services) != len(shared.Services) {
		t.Fatalf("service count = %d, want %d", len(tsModel.Services), len(shared.Services))
	}

	sharedService := shared.Services[0]
	tsService := tsModel.Services[0]
	if got, want := tsService.Namespace, sharedService.Namespace; got != want {
		t.Fatalf("Namespace = %q, want %q", got, want)
	}
	if len(tsService.Icons) != 1 || tsService.Icons[0].GetSrc() != sharedService.Icons[0].GetSrc() {
		t.Fatalf("service icons = %+v, want %+v", tsService.Icons, sharedService.Icons)
	}

	visible := findTypeScriptMethodByProtoName(t, tsService, "Visible")
	sharedVisible := findSharedMethodByProtoName(t, sharedService, "Visible")
	assertTypeScriptMethodPreservesSharedFields(t, visible, sharedVisible)
	if visible.Annotations == nil {
		t.Fatal("Visible annotations should be preserved")
	}
	if !visible.Annotations.GetReadOnlyHint() {
		t.Fatal("Visible read_only_hint should be true")
	}
	if visible.Annotations.OpenWorldHint == nil || visible.Annotations.GetOpenWorldHint() {
		t.Fatal("Visible open_world_hint should be explicitly false")
	}
	if len(visible.Icons) != 1 || visible.Icons[0].GetSrc() != "https://example.com/service.svg" {
		t.Fatalf("Visible should inherit service icon, got %+v", visible.Icons)
	}
	if got, want := visible.TaskSupport, mcpoptionsv1.TaskSupport_TASK_SUPPORT_OPTIONAL; got != want {
		t.Fatalf("Visible TaskSupport = %v, want %v", got, want)
	}

	override := findTypeScriptMethodByProtoName(t, tsService, "OverrideIcon")
	sharedOverride := findSharedMethodByProtoName(t, sharedService, "OverrideIcon")
	assertTypeScriptMethodPreservesSharedFields(t, override, sharedOverride)
	if len(override.Icons) != 1 || override.Icons[0].GetSrc() != "https://example.com/method.png" {
		t.Fatalf("OverrideIcon should preserve method icon, got %+v", override.Icons)
	}
	if got, want := override.TaskSupport, mcpoptionsv1.TaskSupport_TASK_SUPPORT_REQUIRED; got != want {
		t.Fatalf("OverrideIcon TaskSupport = %v, want %v", got, want)
	}
}

func TestTypeScriptModel_ExposesSameFileAndCrossFileRequestResponseRefs(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared-types.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/sharedtypes;sharedtypesv1";`,
			`message SharedRequest {}`,
			`message SharedResponse {}`,
			"",
		}, "\n"),
		"test/v1/service-file.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/servicefile;servicefilev1";`,
			`import "test/v1/shared-types.proto";`,
			`message LocalRequest {}`,
			`message LocalResponse {}`,
			`service ServiceAPI {`,
			`  rpc UseLocal(LocalRequest) returns (LocalResponse);`,
			`  rpc UseShared(SharedRequest) returns (SharedResponse);`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/service-file.proto")
	file := plugin.FilesByPath["test/v1/service-file.proto"]
	if file == nil {
		t.Fatal("service-file proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageTypeScript})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	tsModel, err := CollectTypeScriptFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectTypeScriptFileModel: %v", err)
	}

	service := tsModel.Services[0]
	local := findTypeScriptMethodByProtoName(t, service, "UseLocal")
	assertTypeScriptTypeRef(t, local.Input, typeScriptTypeRefWant{
		protoPath:    "test/v1/service-file.proto",
		current:      true,
		typeName:     "LocalRequest",
		schemaName:   "LocalRequestSchema",
		registryName: "file_test_v1_service_file",
	})
	assertTypeScriptTypeRef(t, local.Output, typeScriptTypeRefWant{
		protoPath:    "test/v1/service-file.proto",
		current:      true,
		typeName:     "LocalResponse",
		schemaName:   "LocalResponseSchema",
		registryName: "file_test_v1_service_file",
	})

	crossFile := findTypeScriptMethodByProtoName(t, service, "UseShared")
	assertTypeScriptTypeRef(t, crossFile.Input, typeScriptTypeRefWant{
		protoPath:    "test/v1/shared-types.proto",
		current:      false,
		typeName:     "SharedRequest",
		schemaName:   "SharedRequestSchema",
		registryName: "file_test_v1_shared_types",
	})
	assertTypeScriptTypeRef(t, crossFile.Output, typeScriptTypeRefWant{
		protoPath:    "test/v1/shared-types.proto",
		current:      false,
		typeName:     "SharedResponse",
		schemaName:   "SharedResponseSchema",
		registryName: "file_test_v1_shared_types",
	})

	if len(tsModel.Imports) != 1 {
		t.Fatalf("import count = %d, want 1", len(tsModel.Imports))
	}
	if got, want := tsModel.Imports[0].ProtoPath, "test/v1/shared-types.proto"; got != want {
		t.Fatalf("Imports[0].ProtoPath = %q, want %q", got, want)
	}
	if got, want := tsModel.Imports[0].ModuleSpecifier, "./shared_types_pb.js"; got != want {
		t.Fatalf("Imports[0].ModuleSpecifier = %q, want %q", got, want)
	}
}

func TestCollectTypeScriptFileModel_RejectsNonTypeScriptTarget(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	_, err = CollectTypeScriptFileModel(file, shared)
	if err == nil {
		t.Fatal("CollectTypeScriptFileModel unexpectedly succeeded for go target")
	}
	if !strings.Contains(err.Error(), "typescript model requires lang=typescript") {
		t.Fatalf("CollectTypeScriptFileModel error = %v, want non-TypeScript target rejection", err)
	}
}

func assertTypeScriptMethodPreservesSharedFields(t *testing.T, got TypeScriptMethodModel, want MethodModel) {
	t.Helper()

	if got.ToolName != want.Name {
		t.Fatalf("%s ToolName = %q, want %q", got.ProtoName, got.ToolName, want.Name)
	}
	if got.InputSchemaJSON != want.InputSchemaJSON {
		t.Fatalf("%s InputSchemaJSON mismatch: got %q want %q", got.ProtoName, got.InputSchemaJSON, want.InputSchemaJSON)
	}
	if got.OutputSchemaJSON != want.OutputSchemaJSON {
		t.Fatalf("%s OutputSchemaJSON mismatch: got %q want %q", got.ProtoName, got.OutputSchemaJSON, want.OutputSchemaJSON)
	}
	if got.Annotations != want.Annotations {
		t.Fatalf("%s Annotations pointer = %p, want shared pointer %p", got.ProtoName, got.Annotations, want.Annotations)
	}
	if !reflect.DeepEqual(got.Icons, want.Icons) {
		t.Fatalf("%s Icons = %+v, want %+v", got.ProtoName, got.Icons, want.Icons)
	}
	if got.TaskSupport != want.TaskSupport {
		t.Fatalf("%s TaskSupport = %v, want %v", got.ProtoName, got.TaskSupport, want.TaskSupport)
	}
}

type typeScriptTypeRefWant struct {
	protoPath    string
	current      bool
	typeName     string
	schemaName   string
	registryName string
}

func assertTypeScriptTypeRef(t *testing.T, got TypeScriptTypeRef, want typeScriptTypeRefWant) {
	t.Helper()

	if got.TypeName != want.typeName {
		t.Fatalf("TypeName = %q, want %q", got.TypeName, want.typeName)
	}
	if got.SchemaName != want.schemaName {
		t.Fatalf("%s SchemaName = %q, want %q", got.TypeName, got.SchemaName, want.schemaName)
	}
	if got.Owner.ProtoPath != want.protoPath {
		t.Fatalf("%s Owner.ProtoPath = %q, want %q", got.TypeName, got.Owner.ProtoPath, want.protoPath)
	}
	if got.Owner.IsCurrentFile != want.current {
		t.Fatalf("%s Owner.IsCurrentFile = %t, want %t", got.TypeName, got.Owner.IsCurrentFile, want.current)
	}
	if got.RegistryRef.RefName != want.registryName {
		t.Fatalf("%s RegistryRef.RefName = %q, want %q", got.TypeName, got.RegistryRef.RefName, want.registryName)
	}
}

func findTypeScriptMethodByProtoName(t *testing.T, service TypeScriptServiceModel, protoName string) TypeScriptMethodModel {
	t.Helper()

	for _, method := range service.Methods {
		if method.ProtoName == protoName {
			return method
		}
	}

	t.Fatalf("typescript method %q not found", protoName)
	return TypeScriptMethodModel{}
}
