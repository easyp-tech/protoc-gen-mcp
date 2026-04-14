package codegen

import (
	"strings"
	"testing"
)

func TestCollectPythonTypeGraph_CrossFileOwnership(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`message SharedLeaf {`,
			`  string value = 1;`,
			`}`,
			`message SharedRequest {`,
			`  SharedLeaf leaf = 1;`,
			`}`,
			`message SharedResponse {}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`import "test/v1/shared.proto";`,
			`message LocalEnvelope {`,
			`  SharedLeaf leaf = 1;`,
			`}`,
			`service CrossFileAPI {`,
			`  rpc UseShared(SharedRequest) returns (LocalEnvelope);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/service.proto")

	file := plugin.FilesByPath["test/v1/service.proto"]
	if file == nil {
		t.Fatal("service proto file not found in plugin")
	}
	sharedFile := plugin.FilesByPath["test/v1/shared.proto"]
	if sharedFile == nil {
		t.Fatal("shared proto file not found in plugin")
	}

	graph, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err != nil {
		t.Fatalf("CollectPythonTypeGraph: %v", err)
	}

	if got, want := graph.CurrentFile.ProtoPath, "test/v1/service.proto"; got != want {
		t.Fatalf("CurrentFile.ProtoPath = %q, want %q", got, want)
	}
	if got, want := graph.CurrentFile.PublicModule.ModulePath, "test.v1.service_mcp"; got != want {
		t.Fatalf("CurrentFile.PublicModule.ModulePath = %q, want %q", got, want)
	}
	if got, want := graph.CurrentFile.ProtobufModule.ModulePath, "test.v1.service_pb2"; got != want {
		t.Fatalf("CurrentFile.ProtobufModule.ModulePath = %q, want %q", got, want)
	}

	localType := findPythonTypeByProtoName(t, graph, "test.v1.LocalEnvelope")
	if !localType.Owner.IsCurrentFile {
		t.Fatal("LocalEnvelope should be owned by the current file")
	}
	if got, want := localType.Owner.PublicModule.ModuleAlias, pythonPublicModuleAliasForProtoPath(file.Desc.Path(), true); got != want {
		t.Fatalf("LocalEnvelope public owner alias = %q, want %q", got, want)
	}
	if got, want := localType.Owner.ProtobufModule.ModuleAlias, pythonModuleAliasForProtoPath(file.Desc.Path(), true); got != want {
		t.Fatalf("LocalEnvelope protobuf owner alias = %q, want %q", got, want)
	}

	sharedType := findPythonTypeByProtoName(t, graph, "test.v1.SharedRequest")
	if sharedType.Owner.IsCurrentFile {
		t.Fatal("SharedRequest should be marked as cross-file")
	}
	if got, want := sharedType.Owner.ProtoPath, "test/v1/shared.proto"; got != want {
		t.Fatalf("SharedRequest owner proto path = %q, want %q", got, want)
	}
	if got, want := sharedType.Owner.PublicModule.ModuleAlias, pythonPublicModuleAliasForProtoPath(sharedFile.Desc.Path(), false); got != want {
		t.Fatalf("SharedRequest public owner alias = %q, want %q", got, want)
	}
	if got, want := sharedType.Owner.ProtobufModule.ModuleAlias, pythonModuleAlias(sharedFile, false); got != want {
		t.Fatalf("SharedRequest protobuf owner alias = %q, want %q", got, want)
	}

	field := findPythonFieldByProtoName(t, localType, "leaf")
	if got, want := field.Type.ProtoFullName, "test.v1.SharedLeaf"; got != want {
		t.Fatalf("field type proto full name = %q, want %q", got, want)
	}
	if field.Type.Owner.IsCurrentFile {
		t.Fatal("cross-file field type should not be marked current-file owned")
	}
	if got, want := field.Type.Owner.PublicModule.ModulePath, "test.v1.shared_mcp"; got != want {
		t.Fatalf("field type public owner module path = %q, want %q", got, want)
	}
	if got, want := field.Type.Owner.ProtobufModule.ModulePath, "test.v1.shared_pb2"; got != want {
		t.Fatalf("field type protobuf owner module path = %q, want %q", got, want)
	}
	if len(graph.Imports) != 1 {
		t.Fatalf("import count = %d, want 1", len(graph.Imports))
	}
	if got, want := graph.Imports[0].PublicModule.ModuleAlias, pythonPublicModuleAliasForProtoPath(sharedFile.Desc.Path(), false); got != want {
		t.Fatalf("import public alias = %q, want %q", got, want)
	}
	if got, want := graph.Imports[0].ProtobufModule.ModuleAlias, pythonModuleAlias(sharedFile, false); got != want {
		t.Fatalf("import protobuf alias = %q, want %q", got, want)
	}
}

func TestCollectPythonTypeGraph_FailsOnNameCollision(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/collision.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/collision;collisionv1";`,
			`message OuterInner {}`,
			`message Outer {`,
			`  message Inner {}`,
			`  Inner inner = 1;`,
			`}`,
			`message CollisionResponse {}`,
			`service CollisionAPI {`,
			`  rpc Check(Outer) returns (OuterInner);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/collision.proto")

	file := plugin.FilesByPath["test/v1/collision.proto"]
	if file == nil {
		t.Fatal("collision proto file not found in plugin")
	}

	_, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err == nil {
		t.Fatal("CollectPythonTypeGraph unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), `python public type name collision for "OuterInner"`) {
		t.Fatalf("CollectPythonTypeGraph error = %v, want collision diagnostic", err)
	}
}

func TestCollectPythonTypeGraph_TracksOneofVariantsAndPresence(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/oneof.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/oneof;oneofv1";`,
			`message VariantMessage {`,
			`  string note = 1;`,
			`}`,
			`message ShapeRequest {`,
			`  optional string nickname = 1;`,
			`  oneof selector {`,
			`    string city_alias = 2;`,
			`    int64 city_id = 3;`,
			`    VariantMessage city_details = 4;`,
			`  }`,
			`}`,
			`message ShapeResponse {}`,
			`service ShapeAPI {`,
			`  rpc Describe(ShapeRequest) returns (ShapeResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/oneof.proto")

	file := plugin.FilesByPath["test/v1/oneof.proto"]
	if file == nil {
		t.Fatal("oneof proto file not found in plugin")
	}

	graph, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err != nil {
		t.Fatalf("CollectPythonTypeGraph: %v", err)
	}

	shape := findPythonTypeByProtoName(t, graph, "test.v1.ShapeRequest")
	nickname := findPythonFieldByProtoName(t, shape, "nickname")
	if !nickname.HasPresence {
		t.Fatal("optional field should track presence")
	}
	if nickname.OneofProtoName != "" {
		t.Fatalf("optional field should not belong to oneof, got %q", nickname.OneofProtoName)
	}

	selector := findPythonOneofByProtoName(t, shape, "selector")
	if got, want := selector.WrapperName, "ShapeRequestSelectorVariant"; got != want {
		t.Fatalf("selector wrapper name = %q, want %q", got, want)
	}
	if len(selector.Variants) != 3 {
		t.Fatalf("selector variant count = %d, want 3", len(selector.Variants))
	}

	cityID := findPythonVariantByProtoName(t, selector, "city_id")
	if !cityID.HasPresence {
		t.Fatal("oneof variant should track presence")
	}
	if got, want := cityID.WrapperName, "ShapeRequestSelectorCityIdVariant"; got != want {
		t.Fatalf("city_id wrapper name = %q, want %q", got, want)
	}
	if got, want := cityID.Type.Scalar, PythonScalarInt64; got != want {
		t.Fatalf("city_id scalar = %q, want %q", got, want)
	}

	cityDetails := findPythonVariantByProtoName(t, selector, "city_details")
	if got, want := cityDetails.Type.ProtoFullName, "test.v1.VariantMessage"; got != want {
		t.Fatalf("city_details type = %q, want %q", got, want)
	}
}

func TestCollectPythonTypeGraph_ClassifiesSupportedWKTs(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/wkts.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/wkts;wktsv1";`,
			`import "google/protobuf/any.proto";`,
			`import "google/protobuf/duration.proto";`,
			`import "google/protobuf/timestamp.proto";`,
			`import "google/protobuf/wrappers.proto";`,
			`message WKTRequest {`,
			`  google.protobuf.Timestamp observed_at = 1;`,
			`  google.protobuf.Duration ttl = 2;`,
			`  google.protobuf.Any detail = 3;`,
			`  google.protobuf.StringValue note = 4;`,
			`}`,
			`message WKTResponse {}`,
			`service WKTAPI {`,
			`  rpc Describe(WKTRequest) returns (WKTResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/wkts.proto")

	file := plugin.FilesByPath["test/v1/wkts.proto"]
	if file == nil {
		t.Fatal("wkt proto file not found in plugin")
	}

	graph, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err != nil {
		t.Fatalf("CollectPythonTypeGraph: %v", err)
	}

	req := findPythonTypeByProtoName(t, graph, "test.v1.WKTRequest")
	if got, want := findPythonFieldByProtoName(t, req, "observed_at").Type.WellKnownType, PythonWellKnownTypeTimestamp; got != want {
		t.Fatalf("observed_at WKT = %q, want %q", got, want)
	}
	if got, want := findPythonFieldByProtoName(t, req, "ttl").Type.WellKnownType, PythonWellKnownTypeDuration; got != want {
		t.Fatalf("ttl WKT = %q, want %q", got, want)
	}
	if got, want := findPythonFieldByProtoName(t, req, "detail").Type.WellKnownType, PythonWellKnownTypeAny; got != want {
		t.Fatalf("detail WKT = %q, want %q", got, want)
	}
	if got, want := findPythonFieldByProtoName(t, req, "note").Type.WellKnownType, PythonWellKnownTypeStringValue; got != want {
		t.Fatalf("note WKT = %q, want %q", got, want)
	}
}

func TestCollectPythonTypeGraph_AllowsSameFlattenedPublicNameAcrossDifferentOwners(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"left/v1/types.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package left.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/left;leftv1";`,
			`message Outer {`,
			`  message Inner {`,
			`    string value = 1;`,
			`  }`,
			`}`,
			``,
		}, "\n"),
		"right/v1/types.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package right.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/right;rightv1";`,
			`message Outer {`,
			`  message Inner {`,
			`    string value = 1;`,
			`  }`,
			`}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`import "left/v1/types.proto";`,
			`import "right/v1/types.proto";`,
			`message UseBothRequest {`,
			`  left.v1.Outer.Inner left_value = 1;`,
			`  right.v1.Outer.Inner right_value = 2;`,
			`}`,
			`message UseBothResponse {}`,
			`service SharedNamesAPI {`,
			`  rpc UseBoth(UseBothRequest) returns (UseBothResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/service.proto")

	file := plugin.FilesByPath["test/v1/service.proto"]
	if file == nil {
		t.Fatal("service proto file not found in plugin")
	}

	graph, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err != nil {
		t.Fatalf("CollectPythonTypeGraph: %v", err)
	}

	leftInner := findPythonTypeByProtoName(t, graph, "left.v1.Outer.Inner")
	rightInner := findPythonTypeByProtoName(t, graph, "right.v1.Outer.Inner")
	if got, want := leftInner.PublicName, "OuterInner"; got != want {
		t.Fatalf("left public name = %q, want %q", got, want)
	}
	if got, want := rightInner.PublicName, "OuterInner"; got != want {
		t.Fatalf("right public name = %q, want %q", got, want)
	}
	if leftInner.Owner.PublicModule.ModulePath == rightInner.Owner.PublicModule.ModulePath {
		t.Fatalf("owner public modules should differ: left=%q right=%q", leftInner.Owner.PublicModule.ModulePath, rightInner.Owner.PublicModule.ModulePath)
	}
}

func TestCollectFileModel_PythonGraphIgnoresHiddenMethodsWithInvalidTypes(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/hidden.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/hidden;hiddenv1";`,
			`import "google/protobuf/type.proto";`,
			`import "mcp/options/v1/options.proto";`,
			`message VisibleRequest {}`,
			`message VisibleResponse {}`,
			`message OuterInner {}`,
			`message Outer {`,
			`  message Inner {}`,
			`  Inner inner = 1;`,
			`}`,
			`message UnsupportedRequest {`,
			`  google.protobuf.Type payload = 1;`,
			`}`,
			`service HiddenAPI {`,
			`  rpc Visible(VisibleRequest) returns (VisibleResponse);`,
			`  rpc HiddenCollision(Outer) returns (OuterInner) {`,
			`    option (mcp.options.v1.method) = { hidden: true };`,
			`  }`,
			`  rpc HiddenUnsupported(UnsupportedRequest) returns (VisibleResponse) {`,
			`    option (mcp.options.v1.method) = { hidden: true };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/hidden.proto")

	file := plugin.FilesByPath["test/v1/hidden.proto"]
	if file == nil {
		t.Fatal("hidden proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if model.PythonTypes == nil {
		t.Fatal("PythonTypes should be populated for python target")
	}

	if len(model.Services) != 1 || len(model.Services[0].Methods) != 1 {
		t.Fatalf("visible method count = %d, want 1", len(model.Services[0].Methods))
	}
	if got, want := model.Services[0].Methods[0].Name, "Visible"; got != want {
		t.Fatalf("visible method name = %q, want %q", got, want)
	}

	findPythonTypeByProtoName(t, *model.PythonTypes, "test.v1.VisibleRequest")
	findPythonTypeByProtoName(t, *model.PythonTypes, "test.v1.VisibleResponse")
	assertPythonTypeAbsent(t, *model.PythonTypes, "test.v1.Outer")
	assertPythonTypeAbsent(t, *model.PythonTypes, "test.v1.Outer.Inner")
	assertPythonTypeAbsent(t, *model.PythonTypes, "test.v1.OuterInner")
	assertPythonTypeAbsent(t, *model.PythonTypes, "test.v1.UnsupportedRequest")
}

func TestCollectPythonTypeGraph_IgnoresHiddenMethodsWithInvalidTypes(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/hidden.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/hidden;hiddenv1";`,
			`import "google/protobuf/type.proto";`,
			`import "mcp/options/v1/options.proto";`,
			`message VisibleRequest {}`,
			`message VisibleResponse {}`,
			`message OuterInner {}`,
			`message Outer {`,
			`  message Inner {}`,
			`  Inner inner = 1;`,
			`}`,
			`message UnsupportedRequest {`,
			`  google.protobuf.Type payload = 1;`,
			`}`,
			`service HiddenAPI {`,
			`  rpc Visible(VisibleRequest) returns (VisibleResponse);`,
			`  rpc HiddenCollision(Outer) returns (OuterInner) {`,
			`    option (mcp.options.v1.method) = { hidden: true };`,
			`  }`,
			`  rpc HiddenUnsupported(UnsupportedRequest) returns (VisibleResponse) {`,
			`    option (mcp.options.v1.method) = { hidden: true };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/hidden.proto")

	file := plugin.FilesByPath["test/v1/hidden.proto"]
	if file == nil {
		t.Fatal("hidden proto file not found in plugin")
	}

	graph, err := CollectPythonTypeGraph(file, PythonRuntimeGoogleProtobuf)
	if err != nil {
		t.Fatalf("CollectPythonTypeGraph: %v", err)
	}

	findPythonTypeByProtoName(t, graph, "test.v1.VisibleRequest")
	findPythonTypeByProtoName(t, graph, "test.v1.VisibleResponse")
	assertPythonTypeAbsent(t, graph, "test.v1.Outer")
	assertPythonTypeAbsent(t, graph, "test.v1.Outer.Inner")
	assertPythonTypeAbsent(t, graph, "test.v1.OuterInner")
	assertPythonTypeAbsent(t, graph, "test.v1.UnsupportedRequest")
}

func findPythonTypeByProtoName(t *testing.T, graph PythonTypeGraph, protoFullName string) PythonType {
	t.Helper()

	for _, typ := range graph.Types {
		if typ.ProtoFullName == protoFullName {
			return typ
		}
	}

	t.Fatalf("python type %q not found", protoFullName)
	return PythonType{}
}

func findPythonFieldByProtoName(t *testing.T, typ PythonType, protoName string) PythonField {
	t.Helper()

	for _, field := range typ.Fields {
		if field.ProtoName == protoName {
			return field
		}
	}

	t.Fatalf("python field %q not found in %q", protoName, typ.ProtoFullName)
	return PythonField{}
}

func findPythonOneofByProtoName(t *testing.T, typ PythonType, protoName string) PythonOneof {
	t.Helper()

	for _, oneof := range typ.Oneofs {
		if oneof.ProtoName == protoName {
			return oneof
		}
	}

	t.Fatalf("python oneof %q not found in %q", protoName, typ.ProtoFullName)
	return PythonOneof{}
}

func findPythonVariantByProtoName(t *testing.T, oneof PythonOneof, protoName string) PythonOneofVariant {
	t.Helper()

	for _, variant := range oneof.Variants {
		if variant.ProtoName == protoName {
			return variant
		}
	}

	t.Fatalf("python oneof variant %q not found in %q", protoName, oneof.ProtoName)
	return PythonOneofVariant{}
}

func assertPythonTypeAbsent(t *testing.T, graph PythonTypeGraph, protoFullName string) {
	t.Helper()

	for _, typ := range graph.Types {
		if typ.ProtoFullName == protoFullName {
			t.Fatalf("python type %q should be absent", protoFullName)
		}
	}
}
