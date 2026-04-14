package codegen

import (
	"reflect"
	"strings"
	"testing"
)

func TestJVMModel_StructuralIRDoesNotContainRawProtogenDescriptors(t *testing.T) {
	disallowed := map[string]struct{}{
		"google.golang.org/protobuf/compiler/protogen.Enum":    {},
		"google.golang.org/protobuf/compiler/protogen.Field":   {},
		"google.golang.org/protobuf/compiler/protogen.File":    {},
		"google.golang.org/protobuf/compiler/protogen.Message": {},
		"google.golang.org/protobuf/compiler/protogen.Method":  {},
		"google.golang.org/protobuf/compiler/protogen.Service": {},
	}

	assertIRTypeHasNoDisallowedFields(t, reflect.TypeOf(JVMFileModel{}), disallowed)
}

func TestJVMModel_StructuralIRDoesNotContainSDKTypes(t *testing.T) {
	assertIRTypeHasNoPackageSubstring(t, reflect.TypeOf(JVMFileModel{}), "modelcontextprotocol")
}

func TestCollectJVMFileModel_PreservesSharedToolSemantics(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageKotlin})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	jvm, err := CollectJVMFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel: %v", err)
	}

	if got, want := jvm.Language, LanguageKotlin; got != want {
		t.Fatalf("Language = %q, want %q", got, want)
	}
	if got, want := jvm.ProtoPath, shared.ProtoPath; got != want {
		t.Fatalf("ProtoPath = %q, want %q", got, want)
	}
	if got, want := jvm.GeneratedFilenamePrefix, shared.GeneratedFilenamePrefix; got != want {
		t.Fatalf("GeneratedFilenamePrefix = %q, want %q", got, want)
	}
	if got, want := jvm.ProtoPackage, "internal.testproto.example.v1"; got != want {
		t.Fatalf("ProtoPackage = %q, want %q", got, want)
	}
	if len(jvm.Services) != len(shared.Services) {
		t.Fatalf("service count = %d, want %d", len(jvm.Services), len(shared.Services))
	}

	sharedService := shared.Services[0]
	jvmService := jvm.Services[0]
	if got, want := jvmService.Namespace, sharedService.Namespace; got != want {
		t.Fatalf("Namespace = %q, want %q", got, want)
	}
	if got, want := jvmService.HandlerName, "ExampleAPIToolHandler"; got != want {
		t.Fatalf("HandlerName = %q, want %q", got, want)
	}
	if got, want := jvmService.RegisterName, "registerExampleAPITools"; got != want {
		t.Fatalf("RegisterName = %q, want %q", got, want)
	}
	if len(jvmService.Methods) != len(sharedService.Methods) {
		t.Fatalf("method count = %d, want %d", len(jvmService.Methods), len(sharedService.Methods))
	}

	sharedMethods := make(map[string]MethodModel, len(sharedService.Methods))
	for _, method := range sharedService.Methods {
		sharedMethods[method.ProtoName] = method
	}
	createReport := findJVMMethodByProtoName(t, jvmService, "CreateReport")
	sharedCreate := sharedMethods["CreateReport"]
	if got, want := createReport.ToolName, sharedCreate.Name; got != want {
		t.Fatalf("ToolName = %q, want %q", got, want)
	}
	if got, want := createReport.MethodName, "createReport"; got != want {
		t.Fatalf("MethodName = %q, want %q", got, want)
	}
	if got, want := createReport.SchemaConst, "EXAMPLE_API_CREATE_REPORT"; got != want {
		t.Fatalf("SchemaConst = %q, want %q", got, want)
	}
	if got, want := createReport.InputSchemaJSON, sharedCreate.InputSchemaJSON; got != want {
		t.Fatalf("InputSchemaJSON mismatch: got %q want %q", got, want)
	}
	if got, want := createReport.OutputSchemaJSON, sharedCreate.OutputSchemaJSON; got != want {
		t.Fatalf("OutputSchemaJSON mismatch: got %q want %q", got, want)
	}
	if got, want := createReport.Input.PublicName, "CreateReportRequest"; got != want {
		t.Fatalf("Input.PublicName = %q, want %q", got, want)
	}
	if !createReport.Input.Owner.IsCurrentFile {
		t.Fatal("Input owner should be current file")
	}

	health := findJVMMethodByProtoName(t, jvmService, "Ping")
	if got, want := health.ToolName, "Health"; got != want {
		t.Fatalf("Ping ToolName = %q, want %q", got, want)
	}
	if got, want := health.MethodName, "ping"; got != want {
		t.Fatalf("Ping MethodName = %q, want %q", got, want)
	}
}

func TestCollectJVMFileModel_PreservesAnnotationsAndIcons(t *testing.T) {
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
			`      annotations: { read_only_hint: true open_world_hint: false }`,
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

	shared, err := CollectFileModel(file, Options{Language: LanguageJava})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	jvm, err := CollectJVMFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel: %v", err)
	}

	service := jvm.Services[0]
	visible := findJVMMethodByProtoName(t, service, "Visible")
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

	override := findJVMMethodByProtoName(t, service, "OverrideIcon")
	if len(override.Icons) != 1 || override.Icons[0].GetSrc() != "https://example.com/method.png" {
		t.Fatalf("OverrideIcon should preserve method icon, got %+v", override.Icons)
	}
}

func TestCollectJVMFileModel_TracksRequirednessNullabilityAndOneofShape(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageJava})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	jvm, err := CollectJVMFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel: %v", err)
	}

	createReq := findJVMTypeByProtoName(t, jvm.Types, "internal.testproto.example.v1.CreateReportRequest")
	city := findJVMFieldByProtoName(t, createReq, "city")
	if !city.IsSchemaRequired {
		t.Fatal("singular non-optional field should be schema-required")
	}
	units := findJVMFieldByProtoName(t, createReq, "units")
	if !units.HasPresence {
		t.Fatal("proto3 optional field should track presence")
	}
	if units.IsSchemaRequired {
		t.Fatal("proto3 optional field should not be schema-required")
	}
	labels := findJVMFieldByProtoName(t, createReq, "labels")
	if !labels.IsRepeated || labels.IsSchemaRequired {
		t.Fatalf("repeated field shape mismatch: repeated=%t required=%t", labels.IsRepeated, labels.IsSchemaRequired)
	}

	advanced := findJVMTypeByProtoName(t, jvm.Types, "internal.testproto.example.v1.DescribeAdvancedShapesRequest")
	mapField := findJVMFieldByProtoName(t, advanced, "labels")
	if !mapField.IsMap {
		t.Fatal("labels should be a map field")
	}
	if got, want := mapField.MapKeyScalar, JVMScalarString; got != want {
		t.Fatalf("labels map key scalar = %q, want %q", got, want)
	}
	if mapField.MapValue == nil || mapField.MapValue.Scalar != JVMScalarString {
		t.Fatalf("labels map value = %+v, want string scalar", mapField.MapValue)
	}
	if mapField.IsSchemaRequired {
		t.Fatal("map field should not be schema-required")
	}
	observedAt := findJVMFieldByProtoName(t, advanced, "observed_at")
	if got, want := observedAt.Type.WellKnownType, JVMWellKnownTypeTimestamp; got != want {
		t.Fatalf("observed_at WKT = %q, want %q", got, want)
	}

	selector := findJVMOneofByProtoName(t, advanced, "selector")
	if got, want := selector.WrapperName, "DescribeAdvancedShapesRequestSelectorVariant"; got != want {
		t.Fatalf("selector wrapper = %q, want %q", got, want)
	}
	if len(selector.Variants) != 3 {
		t.Fatalf("selector variant count = %d, want 3", len(selector.Variants))
	}
	cityID := findJVMVariantByProtoName(t, selector, "city_id")
	if got, want := cityID.WrapperName, "DescribeAdvancedShapesRequestSelectorCityIdVariant"; got != want {
		t.Fatalf("city_id wrapper = %q, want %q", got, want)
	}
	if got, want := cityID.Type.Scalar, JVMScalarInt64; got != want {
		t.Fatalf("city_id scalar = %q, want %q", got, want)
	}

	cityIDField := findJVMFieldByProtoName(t, advanced, "city_id")
	if got, want := cityIDField.OneofProtoName, "selector"; got != want {
		t.Fatalf("city_id oneof = %q, want %q", got, want)
	}
	if cityIDField.IsSchemaRequired {
		t.Fatal("oneof field should not be schema-required")
	}
	if got, want := cityIDField.VariantWrapperName, cityID.WrapperName; got != want {
		t.Fatalf("city_id variant wrapper = %q, want %q", got, want)
	}
}

func TestCollectJVMFileModel_RejectsNonJVMTarget(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	shared, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	_, err = CollectJVMFileModel(file, shared)
	if err == nil {
		t.Fatal("CollectJVMFileModel unexpectedly succeeded for go target")
	}
	if !strings.Contains(err.Error(), "jvm model requires lang=kotlin or lang=java") {
		t.Fatalf("CollectJVMFileModel error = %v, want non-JVM target rejection", err)
	}
}

func findJVMMethodByProtoName(t *testing.T, service JVMServiceModel, protoName string) JVMMethodModel {
	t.Helper()

	for _, method := range service.Methods {
		if method.ProtoName == protoName {
			return method
		}
	}

	t.Fatalf("jvm method %q not found", protoName)
	return JVMMethodModel{}
}

func findJVMTypeByProtoName(t *testing.T, graph JVMTypeGraph, protoFullName string) JVMType {
	t.Helper()

	for _, typ := range graph.Types {
		if typ.ProtoFullName == protoFullName {
			return typ
		}
	}

	t.Fatalf("jvm type %q not found", protoFullName)
	return JVMType{}
}

func findJVMFieldByProtoName(t *testing.T, typ JVMType, protoName string) JVMField {
	t.Helper()

	for _, field := range typ.Fields {
		if field.ProtoName == protoName {
			return field
		}
	}

	t.Fatalf("jvm field %q not found in %q", protoName, typ.ProtoFullName)
	return JVMField{}
}

func findJVMOneofByProtoName(t *testing.T, typ JVMType, protoName string) JVMOneof {
	t.Helper()

	for _, oneof := range typ.Oneofs {
		if oneof.ProtoName == protoName {
			return oneof
		}
	}

	t.Fatalf("jvm oneof %q not found in %q", protoName, typ.ProtoFullName)
	return JVMOneof{}
}

func findJVMVariantByProtoName(t *testing.T, oneof JVMOneof, protoName string) JVMOneofVariant {
	t.Helper()

	for _, variant := range oneof.Variants {
		if variant.ProtoName == protoName {
			return variant
		}
	}

	t.Fatalf("jvm oneof variant %q not found in %q", protoName, oneof.ProtoName)
	return JVMOneofVariant{}
}

func assertIRTypeHasNoPackageSubstring(t *testing.T, typ reflect.Type, disallowed string) {
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
			if strings.Contains(baseType.PkgPath(), disallowed) {
				t.Fatalf("%s.%s uses disallowed package %q in field type %s", current.Name(), field.Name, disallowed, fieldType)
			}
			walk(fieldType)
		}
	}

	walk(typ)
}
