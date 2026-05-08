package codegen

import (
	"strings"
	"testing"
)

func TestPythonRenderer_EmitsDataclassPublicAPI(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"class _UnsetType:",
		"UNSET = _UnsetType()",
		"ToolRequestContext = mcp.server.session.ServerSession",
		"class ReportStatus(enum.IntEnum):",
		"@dataclass(slots=True)",
		"class ReportDetails:",
		"class CreateReportRequest:",
		"class CreateReportResponse:",
		"class ExampleAPIToolHandler(Protocol):",
		"def create_report(self, ctx: ToolRequestContext, req: CreateReportRequest) -> CreateReportResponse | Awaitable[CreateReportResponse]:",
		"def ping(self, ctx: ToolRequestContext, req: PingRequest) -> PingResponse | Awaitable[PingResponse]:",
		"def describe_advanced_shapes(self, ctx: ToolRequestContext, req: DescribeAdvancedShapesRequest) -> DescribeAdvancedShapesResponse | Awaitable[DescribeAdvancedShapesResponse]:",
		"def describe_scalar_shapes(self, ctx: ToolRequestContext, req: DescribeScalarShapesRequest) -> DescribeScalarShapesResponse | Awaitable[DescribeScalarShapesResponse]:",
		"def hidden_thing(self, ctx: ToolRequestContext, req: HiddenThingRequest) -> HiddenThingResponse | Awaitable[HiddenThingResponse]:",
		"EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON =",
		"EXAMPLE_API_HEALTH_OUTPUT_SCHEMA_JSON =",
		"def register_example_api_tools(server: mcp.server.lowlevel.Server, impl: ExampleAPIToolHandler, *, namespace: str | None = None) -> None:",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing snippet %q\n%s", snippet, generated)
		}
	}
	notWantSnippets := []string{
		"    REPORT_STATUS_NONE = 0",
		"def create_report(self, ctx: ToolRequestContext, req: example_pb2.CreateReportRequest)",
		"def create_report(self, ctx: ToolRequestContext, req: CreateReportRequest) -> example_pb2.CreateReportResponse | Awaitable[example_pb2.CreateReportResponse]:",
		"def ping(self, ctx: ToolRequestContext, req: example_pb2.PingRequest)",
		"def ping(self, ctx: ToolRequestContext, req: PingRequest) -> example_pb2.PingResponse | Awaitable[example_pb2.PingResponse]:",
		"def describe_advanced_shapes(self, ctx: ToolRequestContext, req: example_pb2.DescribeAdvancedShapesRequest)",
		"def describe_advanced_shapes(self, ctx: ToolRequestContext, req: DescribeAdvancedShapesRequest) -> example_pb2.DescribeAdvancedShapesResponse | Awaitable[example_pb2.DescribeAdvancedShapesResponse]:",
		"def describe_scalar_shapes(self, ctx: ToolRequestContext, req: example_pb2.DescribeScalarShapesRequest)",
		"def describe_scalar_shapes(self, ctx: ToolRequestContext, req: DescribeScalarShapesRequest) -> example_pb2.DescribeScalarShapesResponse | Awaitable[example_pb2.DescribeScalarShapesResponse]:",
		"def hidden_thing(self, ctx: ToolRequestContext, req: example_pb2.HiddenThingRequest)",
		"def hidden_thing(self, ctx: ToolRequestContext, req: HiddenThingRequest) -> example_pb2.HiddenThingResponse | Awaitable[example_pb2.HiddenThingResponse]:",
	}
	for _, snippet := range notWantSnippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated python must not retain pb2 public API snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_EmitsProtobufHandlerPublicAPI(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
		PythonHandler: PythonHandlerProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"ToolRequestContext = mcp.server.session.ServerSession",
		"class ExampleAPIToolHandler(Protocol):",
		"def create_report(self, ctx: ToolRequestContext, req: example_pb2.CreateReportRequest) -> example_pb2.CreateReportResponse | Awaitable[example_pb2.CreateReportResponse]:",
		"def ping(self, ctx: ToolRequestContext, req: example_pb2.PingRequest) -> example_pb2.PingResponse | Awaitable[example_pb2.PingResponse]:",
		"def describe_advanced_shapes(self, ctx: ToolRequestContext, req: example_pb2.DescribeAdvancedShapesRequest) -> example_pb2.DescribeAdvancedShapesResponse | Awaitable[example_pb2.DescribeAdvancedShapesResponse]:",
		"def register_example_api_tools(server: mcp.server.lowlevel.Server, impl: ExampleAPIToolHandler, *, namespace: str | None = None) -> None:",
		"from_pb=_identity,",
		"to_pb=_identity,",
		"EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON =",
		"EXAMPLE_API_HEALTH_OUTPUT_SCHEMA_JSON =",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing protobuf handler snippet %q\n%s", snippet, generated)
		}
	}
	notWantSnippets := []string{
		"UNSET = _UnsetType()",
		"@dataclass(slots=True)",
		"class CreateReportRequest:",
		"class CreateReportResponse:",
		"def _from_pb_create_report_request",
		"def _to_pb_create_report_response",
		"def create_report(self, ctx: ToolRequestContext, req: CreateReportRequest)",
	}
	for _, snippet := range notWantSnippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated python must not retain dataclass handler snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonAndGoRenderersShareContractModel(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	goModel, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel go: %v", err)
	}
	pythonModel, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel python: %v", err)
	}

	if len(goModel.Services) != len(pythonModel.Services) {
		t.Fatalf("service count mismatch: go=%d python=%d", len(goModel.Services), len(pythonModel.Services))
	}

	for i := range goModel.Services {
		goService := goModel.Services[i]
		pythonService := pythonModel.Services[i]

		if goService.Namespace != pythonService.Namespace {
			t.Fatalf("service %d namespace mismatch: go=%q python=%q", i, goService.Namespace, pythonService.Namespace)
		}
		if len(goService.Methods) != len(pythonService.Methods) {
			t.Fatalf("service %q method count mismatch: go=%d python=%d", goService.ProtoName, len(goService.Methods), len(pythonService.Methods))
		}

		for j := range goService.Methods {
			goMethod := goService.Methods[j]
			pythonMethod := pythonService.Methods[j]
			if goMethod.Name != pythonMethod.Name {
				t.Fatalf("method %d name mismatch: go=%q python=%q", j, goMethod.Name, pythonMethod.Name)
			}
			if goMethod.Title != pythonMethod.Title {
				t.Fatalf("method %q title mismatch: go=%q python=%q", goMethod.Name, goMethod.Title, pythonMethod.Title)
			}
			if goMethod.Description != pythonMethod.Description {
				t.Fatalf("method %q description mismatch: go=%q python=%q", goMethod.Name, goMethod.Description, pythonMethod.Description)
			}
			if goMethod.InputSchemaJSON != pythonMethod.InputSchemaJSON {
				t.Fatalf("method %q input schema mismatch", goMethod.Name)
			}
			if goMethod.OutputSchemaJSON != pythonMethod.OutputSchemaJSON {
				t.Fatalf("method %q output schema mismatch", goMethod.Name)
			}
			if goMethod.Deprecated != pythonMethod.Deprecated {
				t.Fatalf("method %q deprecated mismatch: go=%t python=%t", goMethod.Name, goMethod.Deprecated, pythonMethod.Deprecated)
			}
		}
	}
}

func TestPythonRenderer_ProtobufHandlerImportsCrossFileProtobufModules(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`message SharedRequest {}`,
			`message SharedResponse {}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`import "test/v1/shared.proto";`,
			`service CrossFileAPI {`,
			`  rpc UseShared(SharedRequest) returns (SharedResponse);`,
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
	sharedPBAlias := pythonModuleAlias(sharedFile, false)
	sharedImport := "from test.v1 import shared_pb2"
	if sharedPBAlias != "shared_pb2" {
		sharedImport += " as " + sharedPBAlias
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
		PythonHandler: PythonHandlerProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/service_mcp.py"))
	wantSnippets := []string{
		sharedImport,
		"def use_shared(self, ctx: ToolRequestContext, req: " + sharedPBAlias + ".SharedRequest) -> " + sharedPBAlias + ".SharedResponse | Awaitable[" + sharedPBAlias + ".SharedResponse]:",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing protobuf cross-file snippet %q\n%s", snippet, generated)
		}
	}
	notWantSnippets := []string{
		"shared_mcp",
		"def _from_pb_",
		"def _to_pb_",
	}
	for _, snippet := range notWantSnippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated python must not retain cross-file dataclass snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_ImportsCrossFilePublicTypes(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`message SharedRequest {}`,
			`message SharedResponse {}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`import "test/v1/shared.proto";`,
			`service CrossFileAPI {`,
			`  rpc UseShared(SharedRequest) returns (SharedResponse);`,
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
	sharedAlias := pythonPublicModuleAliasForProtoPath(sharedFile.Desc.Path(), false)

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/service_mcp.py"))
	wantSnippets := []string{
		"from test.v1 import shared_mcp as " + sharedAlias,
		"req: " + sharedAlias + ".SharedRequest",
		") -> " + sharedAlias + ".SharedResponse",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing cross-file import snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_ImportsCrossFileProtobufModulesForMapperHelpers(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`message SharedDetails {`,
			`  string label = 1;`,
			`}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`import "test/v1/shared.proto";`,
			`message LocalRequest {`,
			`  SharedDetails details = 1;`,
			`}`,
			`message LocalResponse {`,
			`  SharedDetails details = 1;`,
			`}`,
			`service CrossFileMapperAPI {`,
			`  rpc UseShared(LocalRequest) returns (LocalResponse);`,
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
	sharedPublicAlias := pythonPublicModuleAliasForProtoPath(sharedFile.Desc.Path(), false)
	sharedPBAlias := pythonModuleAlias(sharedFile, false)

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/service_mcp.py"))
	wantSnippets := []string{
		"from test.v1 import shared_pb2 as " + sharedPBAlias,
		"def _from_pb_" + sharedPublicAlias + "_shared_details(message: " + sharedPBAlias + ".SharedDetails) ->",
		"def _to_pb_" + sharedPublicAlias + "_shared_details(value:",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing cross-file protobuf mapper import snippet %q\n%s", snippet, generated)
		}
	}
}

func TestValidatePythonMapperHelperNames_FailsOnCurrentAndImportedCollision(t *testing.T) {
	err := validatePythonMapperHelperNames([]PythonType{
		{
			Kind:          PythonTypeKindMessage,
			ProtoFullName: "test.v1.SharedDetails",
			PublicName:    "SharedDetails",
			Owner: PythonTypeOwner{
				IsCurrentFile: true,
			},
		},
		{
			Kind:          PythonTypeKindMessage,
			ProtoFullName: "test.v1.Details",
			PublicName:    "Details",
			Owner: PythonTypeOwner{
				PublicModule: PythonModuleRef{ModuleAlias: "shared"},
			},
		},
	})
	if err == nil {
		t.Fatal("validatePythonMapperHelperNames unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "python mapper helper name collision") {
		t.Fatalf("unexpected helper collision error: %v", err)
	}
}

func TestPythonRenderer_FailsOnMapperHelperNameCollision(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`message Any {`,
			`  string label = 1;`,
			`}`,
			`message CollisionRequest {`,
			`  Any payload = 1;`,
			`}`,
			`message CollisionResponse {}`,
			`service CollisionAPI {`,
			`  rpc UseCollision(CollisionRequest) returns (CollisionResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/service.proto")

	file := plugin.FilesByPath["test/v1/service.proto"]
	if file == nil {
		t.Fatal("service proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}

	err = renderPythonFile(plugin, model)
	if err == nil {
		t.Fatal("renderPythonFile unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "python mapper helper name collision") {
		t.Fatalf("unexpected render error: %v", err)
	}
}

func TestPythonRenderer_DoesNotImportWellKnownTypePublicModules(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	notWantSnippets := []string{
		"from google.protobuf import any_mcp",
		"from google.protobuf import duration_mcp",
		"from google.protobuf import empty_mcp",
		"from google.protobuf import field_mask_mcp",
		"from google.protobuf import struct_mcp",
		"from google.protobuf import timestamp_mcp",
		"from google.protobuf import wrappers_mcp",
	}
	for _, snippet := range notWantSnippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated python must not import nonexistent WKT public module %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_EmitsExplicitOneofWrappers(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"class DescribeAdvancedShapesRequestSelectorVariant:",
		"class DescribeAdvancedShapesRequestSelectorCityAliasVariant(DescribeAdvancedShapesRequestSelectorVariant):",
		"class DescribeAdvancedShapesRequestSelectorCityIdVariant(DescribeAdvancedShapesRequestSelectorVariant):",
		"class DescribeAdvancedShapesRequestSelectorCityDetailsVariant(DescribeAdvancedShapesRequestSelectorVariant):",
		"selector: DescribeAdvancedShapesRequestSelectorVariant | _UnsetType = UNSET",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing explicit oneof wrapper snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_AlignsDataclassRequirednessWithSchemaRequiredFields(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"class CreateReportRequest:",
		"    city: str",
		"    count: int",
		"    details: ReportDetails",
		"    units: str | _UnsetType = UNSET",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing requiredness snippet %q\n%s", snippet, generated)
		}
	}
	if strings.Contains(generated, "    details: ReportDetails | _UnsetType = UNSET") {
		t.Fatalf("generated python must keep schema-required dataclass fields required\n%s", generated)
	}
}

func TestPythonRenderer_UsesEnumAnnotationsWithoutNumericFallback(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/enums.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/enums;enumsv1";`,
			`enum JobStatus {`,
			`  JOB_STATUS_UNSPECIFIED = 0;`,
			`  JOB_STATUS_OK = 1;`,
			`  JOB_STATUS_FAILED = 2;`,
			`}`,
			`message EnumRequest {`,
			`  JobStatus status = 1;`,
			`  optional JobStatus optional_status = 2;`,
			`  repeated JobStatus statuses = 3;`,
			`  map<string, JobStatus> status_by_name = 4;`,
			`}`,
			`message EnumResponse {}`,
			`service EnumAPI {`,
			`  rpc UseEnum(EnumRequest) returns (EnumResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/enums.proto")

	file := plugin.FilesByPath["test/v1/enums.proto"]
	if file == nil {
		t.Fatal("enums proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/enums_mcp.py"))
	wantSnippets := []string{
		"    status: JobStatus",
		"    optional_status: JobStatus | _UnsetType = UNSET",
		"    statuses: list[JobStatus] = field(default_factory=list)",
		"    status_by_name: dict[str, JobStatus] = field(default_factory=dict)",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python must keep enum annotations aligned with the MCP boundary contract: missing %q\n%s", snippet, generated)
		}
	}
	notWantSnippets := []string{
		"    status: JobStatus | int",
		"    optional_status: JobStatus | int | _UnsetType = UNSET",
		"    statuses: list[JobStatus | int] = field(default_factory=list)",
		"    status_by_name: dict[str, JobStatus | int] = field(default_factory=dict)",
	}
	for _, snippet := range notWantSnippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated python must not advertise numeric enum fallback in public annotations: found %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_KeepsOptionalMessageAndWKTFieldsUnsettable(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"    child: RecursiveNode | _UnsetType = UNSET",
		"    observed_at: Timestamp | _UnsetType = UNSET",
		"    ttl: Duration | _UnsetType = UNSET",
		"    payload: Struct | _UnsetType = UNSET",
		"    detail_any: ProtoAny | _UnsetType = UNSET",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python must keep optional message/WKT fields unsettable: missing %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_UsesSafeCurrentModuleImport(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"examples/1-helloworld/proto/hello.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/hello;hellov1";`,
			`message HelloRequest {}`,
			`message HelloResponse {}`,
			`service HelloAPI {`,
			`  rpc SayHello(HelloRequest) returns (HelloResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "examples/1-helloworld/proto/hello.proto")

	file := plugin.FilesByPath["examples/1-helloworld/proto/hello.proto"]
	if file == nil {
		t.Fatal("hello proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "examples/1_helloworld/proto/hello_mcp.py"))
	wantSnippets := []string{
		"try:\n    from . import hello_pb2",
		"except ImportError:\n    import hello_pb2",
		"@dataclass(slots=True)\nclass HelloRequest:",
		"@dataclass(slots=True)\nclass HelloResponse:",
		"def say_hello(self, ctx: ToolRequestContext, req: HelloRequest) -> HelloResponse | Awaitable[HelloResponse]:",
		"request_type=hello_pb2.HelloRequest",
		"response_type=hello_pb2.HelloResponse",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing safe current-module import snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_EmitsPythonBoolLiteralsInAnnotations(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/annotations.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`import "mcp/options/v1/options.proto";`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/annotations;annotationsv1";`,
			`message AnnotatedRequest {}`,
			`message AnnotatedResponse {}`,
			`service AnnotatedAPI {`,
			`  rpc ReadThing(AnnotatedRequest) returns (AnnotatedResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      annotations: {`,
			`        read_only_hint: true`,
			`        idempotent_hint: true`,
			`        open_world_hint: true`,
			`      }`,
			`    };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/annotations.proto")

	file := plugin.FilesByPath["test/v1/annotations.proto"]
	if file == nil {
		t.Fatal("annotations proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/annotations_mcp.py"))
	if !strings.Contains(generated, `annotations={"readOnlyHint": True, "idempotentHint": True, "openWorldHint": True}`) {
		t.Fatalf("generated python must emit Python bool literals for annotations\n%s", generated)
	}
	if strings.Contains(generated, `annotations={"readOnlyHint": true`) || strings.Contains(generated, `annotations={"idempotentHint": true`) {
		t.Fatalf("generated python must not emit JSON bool literals for annotations\n%s", generated)
	}
}

func TestPythonRenderer_ProjectsTaskSupportExecution(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/execution.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`import "mcp/options/v1/options.proto";`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/execution;executionv1";`,
			`message ExecutionRequest {}`,
			`message ExecutionResponse {}`,
			`service ExecutionAPI {`,
			`  rpc OptionalTask(ExecutionRequest) returns (ExecutionResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      execution: { task_support: TASK_SUPPORT_OPTIONAL }`,
			`    };`,
			`  }`,
			`  rpc RequiredTask(ExecutionRequest) returns (ExecutionResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      execution: { task_support: TASK_SUPPORT_REQUIRED }`,
			`    };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/execution.proto")

	file := plugin.FilesByPath["test/v1/execution.proto"]
	if file == nil {
		t.Fatal("execution proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "test/v1/execution_mcp.py"))
	wantSnippets := []string{
		"execution: dict[str, Any] | None = None",
		"def _tool_execution(raw: dict[str, Any] | None) -> mcp.types.ToolExecution | None:",
		"execution=_tool_execution(tool.execution),",
		`execution={"taskSupport": "optional"}`,
		`execution={"taskSupport": "required"}`,
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing execution snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonModuleAlias_AvoidsImportedModuleCollisions(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"a/b_c.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/aliasone;aliasonev1";`,
			`message AliasOne {}`,
			``,
		}, "\n"),
		"a_b/c.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/aliastwo;aliastwov1";`,
			`message AliasTwo {}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/aliassvc;aliassvcv1";`,
			`import "a/b_c.proto";`,
			`import "a_b/c.proto";`,
			`message AliasRequest {`,
			`  test.v1.AliasOne one = 1;`,
			`  test.v1.AliasTwo two = 2;`,
			`}`,
			`message AliasResponse {}`,
			`service AliasAPI {`,
			`  rpc UseAlias(AliasRequest) returns (AliasResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/service.proto")

	first := plugin.FilesByPath["a/b_c.proto"]
	if first == nil {
		t.Fatal("first proto file not found in plugin")
	}
	second := plugin.FilesByPath["a_b/c.proto"]
	if second == nil {
		t.Fatal("second proto file not found in plugin")
	}

	firstAlias := pythonModuleAlias(first, false)
	secondAlias := pythonModuleAlias(second, false)
	if firstAlias == secondAlias {
		t.Fatalf("imported module aliases collided: %q", firstAlias)
	}
}

func TestPythonRenderer_ComposesExistingLowLevelHandlers(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"self.previous_list_tools: Any | None = None",
		"self.previous_call_tool: Any | None = None",
		"registry.previous_list_tools = server.request_handlers.get(mcp.types.ListToolsRequest)",
		"registry.previous_call_tool = server.request_handlers.get(mcp.types.CallToolRequest)",
		"list_request = _build_list_tools_request(request)",
		"previous_result = await current.previous_list_tools(list_request)",
		"meta: dict[str, Any] | None = None",
		"if current.previous_list_tools is not None:",
		"if getattr(list_request.params, \"cursor\", None) is not None:",
		"if previous_result.root.nextCursor is not None:",
		"raise ValueError(\"cannot compose protoc-gen-mcp tools with paginated tools/list handlers\")",
		"meta = getattr(previous_result.root, \"meta\", None)",
		"tools = _merge_tools(previous_tools, current_tools)",
		"mcp.types.ListToolsResult(tools=tools, _meta=meta)",
		"if current.previous_call_tool is not None:",
		"await server.request_handlers[mcp.types.ListToolsRequest](mcp.types.ListToolsRequest())",
		"return await current.previous_call_tool(request)",
		"_invalid_params_error(request.params.name, \"unknown tool\")",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing handler-composition snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_ReservesToolNamesServerWide(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"_RESERVED_TOOL_NAMES_ATTR = \"_protoc_gen_mcp_reserved_tool_names\"",
		"self.reserved_tool_names = _get_reserved_tool_names(server)",
		"def _get_reserved_tool_names(server: mcp.server.lowlevel.Server) -> set[str]:",
		"reserved = getattr(server, _RESERVED_TOOL_NAMES_ATTR, None)",
		"setattr(server, _RESERVED_TOOL_NAMES_ATTR, reserved)",
		"if tool.name in self.reserved_tool_names:",
		"self.reserved_tool_names.add(tool.name)",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing server-wide reservation snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_DetectsMergedDuplicateToolNames(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"def _merge_tools(previous: list[mcp.types.Tool], current: list[mcp.types.Tool]) -> list[mcp.types.Tool]:",
		"if tool.name in names:",
		"raise ValueError(f\"duplicate tool registration: {tool.name}\")",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing merged-duplicate detection snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRenderer_WrapsOutputValidationFailures(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"except jsonschema.ValidationError as error:",
		"raise RuntimeError(f\"mcpruntime: validate output for tool {name!r}: {error}\") from error",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing output-validation wrapper snippet %q\n%s", snippet, generated)
		}
	}
}

func TestPythonRuntime_DispatchesThroughDataclassMappers(t *testing.T) {
	plugin := newExampleProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/example/v1/example.proto"]
	if file == nil {
		t.Fatal("example proto file not found in plugin")
	}

	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}

	generated := string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
	wantSnippets := []string{
		"request_pb = _protojson_to_message(arguments, tool.request_type())",
		"request_dc = tool.from_pb(request_pb)",
		"response_dc = await _maybe_await(tool.handler(context, request_dc))",
		"response_pb = tool.to_pb(response_dc)",
		"from_pb=_from_pb_create_report_request,",
		"to_pb=_to_pb_create_report_response,",
		"payload = json.loads(_message_to_json(response_pb))",
		"return mcp.types.CallToolResult(content=content, structuredContent=payload)",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated python missing dataclass dispatch snippet %q\n%s", snippet, generated)
		}
	}
}
