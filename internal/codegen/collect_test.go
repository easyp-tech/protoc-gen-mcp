package codegen

import (
	"context"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/protoutil"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestCollectFileModel_ExampleAPI(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	if len(model.Services) != 1 {
		t.Fatalf("service count = %d, want 1", len(model.Services))
	}

	service := model.Services[0]
	if got := model.ProtoPath; got != "internal/testproto/example/v1/example.proto" {
		t.Fatalf("ProtoPath = %q, want %q", got, "internal/testproto/example/v1/example.proto")
	}
	if got := model.GeneratedFilenamePrefix; got != "internal/testproto/example/v1/example" {
		t.Fatalf("GeneratedFilenamePrefix = %q, want %q", got, "internal/testproto/example/v1/example")
	}
	if model.Options.Language != LanguageGo {
		t.Fatalf("Options.Language = %q, want %q", model.Options.Language, LanguageGo)
	}
	if got := service.Namespace; got != "example" {
		t.Fatalf("Namespace = %q, want %q", got, "example")
	}
	if got := service.ProtoName; got != "ExampleAPI" {
		t.Fatalf("ProtoName = %q, want %q", got, "ExampleAPI")
	}
	if got := service.ProtoFullName; got != "internal.testproto.example.v1.ExampleAPI" {
		t.Fatalf("ProtoFullName = %q, want %q", got, "internal.testproto.example.v1.ExampleAPI")
	}
	if len(service.Methods) != 5 {
		t.Fatalf("method count = %d, want 5", len(service.Methods))
	}

	method := service.Methods[0]
	if got := method.ProtoName; got != "CreateReport" {
		t.Fatalf("ProtoName = %q, want %q", got, "CreateReport")
	}
	if got := method.ProtoFullName; got != "internal.testproto.example.v1.ExampleAPI.CreateReport" {
		t.Fatalf("ProtoFullName = %q, want %q", got, "internal.testproto.example.v1.ExampleAPI.CreateReport")
	}
	if got := method.Name; got != "CreateReport" {
		t.Fatalf("Method.Name = %q, want %q", got, "CreateReport")
	}
	if got := method.Title; got != "Create report" {
		t.Fatalf("Method.Title = %q, want %q", got, "Create report")
	}
	if got := method.Description; got != "Create a report for a city." {
		t.Fatalf("Method.Description = %q, want %q", got, "Create a report for a city.")
	}
	if method.InputSchemaJSON == "" {
		t.Fatal("Method.InputSchemaJSON is empty")
	}
	if method.OutputSchemaJSON == "" {
		t.Fatal("Method.OutputSchemaJSON is empty")
	}
	if got := method.Input.ProtoDisplayName; got != "CreateReportRequest" {
		t.Fatalf("Input.ProtoDisplayName = %q, want %q", got, "CreateReportRequest")
	}
	if got := method.Input.ProtoFullName; got != "internal.testproto.example.v1.CreateReportRequest" {
		t.Fatalf("Input.ProtoFullName = %q, want %q", got, "internal.testproto.example.v1.CreateReportRequest")
	}
	if got := method.Output.ProtoDisplayName; got != "CreateReportResponse" {
		t.Fatalf("Output.ProtoDisplayName = %q, want %q", got, "CreateReportResponse")
	}
	if got := method.Output.ProtoFullName; got != "internal.testproto.example.v1.CreateReportResponse" {
		t.Fatalf("Output.ProtoFullName = %q, want %q", got, "internal.testproto.example.v1.CreateReportResponse")
	}

	hidden := service.Methods[len(service.Methods)-1]
	if !hidden.Deprecated {
		t.Fatal("HiddenThing should propagate deprecated metadata")
	}
	if !strings.Contains(hidden.InputSchemaJSON, `"deprecated":true`) {
		t.Fatalf("HiddenThing input schema should be marked deprecated: %s", hidden.InputSchemaJSON)
	}
}

func TestCollectFileModel_StructuralIRDoesNotContainRawProtogenDescriptors(t *testing.T) {
	disallowed := map[string]struct{}{
		"google.golang.org/protobuf/compiler/protogen.Enum":    {},
		"google.golang.org/protobuf/compiler/protogen.Field":   {},
		"google.golang.org/protobuf/compiler/protogen.File":    {},
		"google.golang.org/protobuf/compiler/protogen.Message": {},
		"google.golang.org/protobuf/compiler/protogen.Method":  {},
		"google.golang.org/protobuf/compiler/protogen.Service": {},
	}

	assertIRTypeHasNoDisallowedFields(t, reflect.TypeOf(FileModel{}), disallowed)
}

func TestCollectFileModel_StructuralIRIsLanguageNeutral(t *testing.T) {
	assertIRTypeHasNoFieldNames(t, reflect.TypeOf(FileModel{}), map[string]struct{}{
		"GoPackageName":    {},
		"GoImportPath":     {},
		"GoName":           {},
		"GoIdent":          {},
		"GoQualifiedIdent": {},
	})
}

func newExampleProtogenPlugin(t *testing.T) *protogen.Plugin {
	t.Helper()

	return newCompiledProtogenPlugin(t, []string{"internal/testproto/example/v1/example.proto"}, &protocompile.SourceResolver{
		ImportPaths: []string{repoRoot(t)},
	})
}

func TestCollectFileModel_ProtoSyntaxGate(t *testing.T) {
	t.Run("accepts proto3", func(t *testing.T) {
		plugin := newTempProtogenPlugin(t, map[string]string{
			"test/v1/accept.proto": strings.Join([]string{
				`syntax = "proto3";`,
				`package test.v1;`,
				`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/accept;acceptv1";`,
				`message PingRequest {}`,
				`message PingResponse {}`,
				`service AcceptService {`,
				`  rpc Ping(PingRequest) returns (PingResponse);`,
				`}`,
				"",
			}, "\n"),
		}, "test/v1/accept.proto")
		file := plugin.FilesByPath["test/v1/accept.proto"]
		if file == nil {
			t.Fatal("accept proto file not found in plugin")
		}

		model, err := CollectFileModel(file, Options{Language: LanguageGo})
		if err != nil {
			t.Fatalf("CollectFileModel: %v", err)
		}
		if len(model.Services) != 1 {
			t.Fatalf("service count = %d, want 1", len(model.Services))
		}
	})

	t.Run("rejects proto2", func(t *testing.T) {
		plugin := newTempProtogenPlugin(t, map[string]string{
			"test/v1/reject.proto": strings.Join([]string{
				`syntax = "proto2";`,
				`package test.v1;`,
				`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/reject;rejectv1";`,
				`message PingRequest {}`,
				`message PingResponse {}`,
				`service RejectService {`,
				`  rpc Ping(PingRequest) returns (PingResponse);`,
				`}`,
				"",
			}, "\n"),
		}, "test/v1/reject.proto")
		file := plugin.FilesByPath["test/v1/reject.proto"]
		if file == nil {
			t.Fatal("reject proto file not found in plugin")
		}

		_, err := CollectFileModel(file, Options{Language: LanguageGo})
		if err == nil {
			t.Fatal("CollectFileModel unexpectedly succeeded for proto2 file")
		}
		if !strings.Contains(err.Error(), "only proto3 files are supported in MVP") {
			t.Fatalf("CollectFileModel error = %v, want proto3 rejection", err)
		}
	})
}

func TestCollectFileModel_RejectsStreamingRPC(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/streaming.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/streaming;streamingv1";`,
			`message StreamRequest {}`,
			`message StreamResponse {}`,
			`service StreamService {`,
			`  rpc Watch(StreamRequest) returns (stream StreamResponse);`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/streaming.proto")
	file := plugin.FilesByPath["test/v1/streaming.proto"]
	if file == nil {
		t.Fatal("streaming proto file not found in plugin")
	}

	_, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err == nil {
		t.Fatal("CollectFileModel unexpectedly succeeded for streaming RPC")
	}
	if !strings.Contains(err.Error(), "streaming RPC is not supported") {
		t.Fatalf("CollectFileModel error = %v, want streaming rejection", err)
	}
}

func TestCollectFileModel_FiltersHiddenMethodsAndFallsBackToServiceIcons(t *testing.T) {
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
			`    option (mcp.options.v1.method) = { title: "Visible tool" };`,
			`  }`,
			``,
			`  rpc Hidden(Request) returns (Response) {`,
			`    option (mcp.options.v1.method) = { hidden: true };`,
			`  }`,
			``,
			`  rpc Deprecated(Request) returns (Response) {`,
			`    option deprecated = true;`,
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

	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if len(model.Services) != 1 {
		t.Fatalf("service count = %d, want 1", len(model.Services))
	}

	service := model.Services[0]
	if len(service.Methods) != 3 {
		t.Fatalf("method count = %d, want 3 after hidden filtering", len(service.Methods))
	}

	methods := make(map[string]MethodModel, len(service.Methods))
	for _, method := range service.Methods {
		methods[method.Name] = method
	}
	if _, ok := methods["Hidden"]; ok {
		t.Fatal("hidden method should not be collected")
	}

	visible, ok := methods["Visible"]
	if !ok {
		t.Fatal("visible method not collected")
	}
	if len(visible.Icons) != 1 {
		t.Fatalf("visible method icon count = %d, want 1", len(visible.Icons))
	}
	if got := visible.Icons[0].GetSrc(); got != "https://example.com/service.svg" {
		t.Fatalf("visible method icon src = %q, want service fallback", got)
	}

	deprecated, ok := methods["Deprecated"]
	if !ok {
		t.Fatal("deprecated method not collected")
	}
	if !deprecated.Deprecated {
		t.Fatal("deprecated method should propagate deprecated metadata")
	}
	if !strings.Contains(deprecated.InputSchemaJSON, `"deprecated":true`) {
		t.Fatalf("deprecated input schema should be marked deprecated: %s", deprecated.InputSchemaJSON)
	}
	if len(deprecated.Icons) != 1 || deprecated.Icons[0].GetSrc() != "https://example.com/service.svg" {
		t.Fatalf("deprecated method should inherit service icon, got %+v", deprecated.Icons)
	}

	override, ok := methods["OverrideIcon"]
	if !ok {
		t.Fatal("override icon method not collected")
	}
	if len(override.Icons) != 1 {
		t.Fatalf("override method icon count = %d, want 1", len(override.Icons))
	}
	if got := override.Icons[0].GetSrc(); got != "https://example.com/method.png" {
		t.Fatalf("override method icon src = %q, want method-specific icon", got)
	}
}

func sourceFileDescriptors(files []*descriptorpb.FileDescriptorProto, paths ...string) []*descriptorpb.FileDescriptorProto {
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}

	var descriptors []*descriptorpb.FileDescriptorProto
	for _, file := range files {
		if wanted[file.GetName()] {
			descriptors = append(descriptors, file)
		}
	}
	return descriptors
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func newTempProtogenPlugin(t *testing.T, files map[string]string, filesToGenerate ...string) *protogen.Plugin {
	t.Helper()

	if len(filesToGenerate) == 0 {
		t.Fatal("newTempProtogenPlugin requires at least one file to generate")
	}

	return newCompiledProtogenPlugin(t, filesToGenerate, protocompile.CompositeResolver{
		&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(files)},
		&protocompile.SourceResolver{ImportPaths: []string{repoRoot(t)}},
	})
}

func newCompiledProtogenPlugin(t *testing.T, filesToGenerate []string, resolver protocompile.Resolver) *protogen.Plugin {
	t.Helper()

	compiler := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(resolver),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}

	compiled, err := compiler.Compile(context.Background(), filesToGenerate...)
	if err != nil {
		t.Fatalf("compile proto descriptors for %v: %v", filesToGenerate, err)
	}

	compiledDescriptors := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, file := range compiled {
		compiledDescriptors = append(compiledDescriptors, file)
	}
	descriptorProtos := normalizeDescriptorProtos(t, collectDescriptorProtos(compiledDescriptors))

	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate:        append([]string(nil), filesToGenerate...),
		Parameter:             proto.String("paths=source_relative"),
		ProtoFile:             descriptorProtos,
		SourceFileDescriptors: sourceFileDescriptors(descriptorProtos, filesToGenerate...),
		CompilerVersion:       &pluginpb.Version{},
	}

	plugin, err := protogen.Options{}.New(request)
	if err != nil {
		t.Fatalf("protogen.Options.New for %v: %v", filesToGenerate, err)
	}

	return plugin
}

func collectDescriptorProtos(files []protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	visited := make(map[string]bool)
	descriptors := make([]*descriptorpb.FileDescriptorProto, 0, len(files))

	var walk func(protoreflect.FileDescriptor)
	walk = func(file protoreflect.FileDescriptor) {
		if visited[file.Path()] {
			return
		}
		visited[file.Path()] = true

		imports := file.Imports()
		for i := 0; i < imports.Len(); i++ {
			walk(imports.Get(i).FileDescriptor)
		}

		descriptors = append(descriptors, protoutil.ProtoFromFileDescriptor(file))
	}

	for _, file := range files {
		walk(file)
	}

	return descriptors
}

func normalizeDescriptorProtos(t *testing.T, files []*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	t.Helper()

	normalized := make([]*descriptorpb.FileDescriptorProto, 0, len(files))
	for _, file := range files {
		data, err := proto.Marshal(file)
		if err != nil {
			t.Fatalf("marshal descriptor proto %q: %v", file.GetName(), err)
		}

		var clone descriptorpb.FileDescriptorProto
		if err := proto.Unmarshal(data, &clone); err != nil {
			t.Fatalf("unmarshal descriptor proto %q: %v", file.GetName(), err)
		}
		normalized = append(normalized, &clone)
	}

	return normalized
}

func assertIRTypeHasNoDisallowedFields(t *testing.T, typ reflect.Type, disallowed map[string]struct{}) {
	t.Helper()

	visited := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(current reflect.Type) {
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		switch current.Kind() {
		case reflect.Map:
			walk(current.Key())
			walk(current.Elem())
			return
		case reflect.Struct:
		default:
			return
		}

		if visited[current] {
			return
		}
		visited[current] = true

		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			fieldType := field.Type
			baseType := fieldType
			for baseType.Kind() == reflect.Pointer || baseType.Kind() == reflect.Slice || baseType.Kind() == reflect.Array {
				baseType = baseType.Elem()
			}
			if _, ok := disallowed[baseType.PkgPath()+"."+baseType.Name()]; ok {
				t.Fatalf("%s.%s uses disallowed IR field type %s", current.Name(), field.Name, fieldType)
			}
			walk(fieldType)
		}
	}

	walk(typ)
}

func assertIRTypeHasNoFieldNames(t *testing.T, typ reflect.Type, disallowed map[string]struct{}) {
	t.Helper()

	visited := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(current reflect.Type) {
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		switch current.Kind() {
		case reflect.Map:
			walk(current.Key())
			walk(current.Elem())
			return
		case reflect.Struct:
		default:
			return
		}

		if visited[current] {
			return
		}
		visited[current] = true

		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			if _, ok := disallowed[field.Name]; ok {
				t.Fatalf("%s.%s is Go-specific and should not be part of the shared IR", current.Name(), field.Name)
			}
			walk(field.Type)
		}
	}

	walk(typ)
}
