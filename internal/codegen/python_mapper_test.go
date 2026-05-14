package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
)

func renderExamplePythonForMapperTests(t *testing.T) string {
	t.Helper()

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

	return string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
}

func repoRootForPythonMapperTests(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func prepareExamplePythonRuntime(t *testing.T) (tempRoot string, repoRoot string) {
	t.Helper()

	repoRoot = repoRootForPythonMapperTests(t)
	tempRoot = t.TempDir()

	packageDir := filepath.Join(tempRoot, "internal", "testproto", "example", "v1")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", packageDir, err)
	}

	for _, rel := range []string{
		"internal/__init__.py",
		"internal/testproto/__init__.py",
		"internal/testproto/example/__init__.py",
		"internal/testproto/example/v1/__init__.py",
	} {
		path := filepath.Join(tempRoot, filepath.FromSlash(rel))
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	generatedPath := filepath.Join(packageDir, "example_mcp.py")
	if err := os.WriteFile(generatedPath, []byte(renderExamplePythonForMapperTests(t)), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", generatedPath, err)
	}

	examplePB2Path := filepath.Join(repoRoot, "internal", "testproto", "example", "v1", "example_pb2.py")
	examplePB2Content, err := os.ReadFile(examplePB2Path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", examplePB2Path, err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "example_pb2.py"), examplePB2Content, 0o644); err != nil {
		t.Fatalf("WriteFile(example_pb2.py): %v", err)
	}

	return tempRoot, repoRoot
}

func runExamplePythonScript(t *testing.T, script string) {
	t.Helper()

	tempRoot, repoRoot := prepareExamplePythonRuntime(t)
	cmd := pythontest.Command(t, "-c", fmt.Sprintf(
		"from internal.testproto.example.v1 import example_mcp as module\n"+
			"from internal.testproto.example.v1 import example_pb2\n%s",
		script,
	))
	cmd.Dir = tempRoot
	cmd.Env = pythontest.Env(t, "PYTHONPATH="+tempRoot+string(os.PathListSeparator)+repoRoot)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python runtime failed: %v\n%s", err, output)
	}
}

func TestPythonMapper_RoundTripsOptionalAndUnset(t *testing.T) {
	runExamplePythonScript(t, `
request = module.CreateReportRequest(
    city="Paris",
    count=2,
    details=module.ReportDetails(label="today"),
    units=module.UNSET,
    labels=["daily"],
)
request_pb = module._to_pb_create_report_request(request)
assert not request_pb.HasField("units")
assert list(request_pb.labels) == ["daily"]
assert module._from_pb_create_report_request(request_pb) == request

scalar = module.DescribeScalarShapesRequest(
    bool_flag=True,
    text_value="hello",
    bytes_value=b"abc",
    int32_value=-1,
    sint32_value=-2,
    sfixed32_value=-3,
    uint32_value=7,
    fixed32_value=8,
    int64_value=-9,
    sint64_value=-10,
    sfixed64_value=-11,
    uint64_value=12,
    fixed64_value=13,
    float_value=1.25,
    double_value=2.5,
    status=module.ReportStatus.REPORT_STATUS_OK,
    details=module.ReportDetails(label="shape"),
    samples=[1, 2, 3],
    optional_text_value=module.UNSET,
    optional_status=module.ReportStatus.REPORT_STATUS_FAILED,
)
scalar_pb = module._to_pb_describe_scalar_shapes_request(scalar)
assert not scalar_pb.HasField("optional_text_value")
assert scalar_pb.HasField("optional_status")
assert scalar_pb.optional_status == int(module.ReportStatus.REPORT_STATUS_FAILED)
scalar_roundtrip = module._from_pb_describe_scalar_shapes_request(scalar_pb)
assert scalar_roundtrip.optional_text_value is module.UNSET
assert scalar_roundtrip == scalar
`)
}

func TestPythonMapper_RoundTripsExplicitOneofVariants(t *testing.T) {
	runExamplePythonScript(t, `
cases = [
    (
        module.DescribeAdvancedShapesRequest(
            selector=module.DescribeAdvancedShapesRequestSelectorCityAliasVariant(city_alias="msk"),
        ),
        "city_alias",
    ),
    (
        module.DescribeAdvancedShapesRequest(
            selector=module.DescribeAdvancedShapesRequestSelectorCityIdVariant(city_id=1234567890123),
        ),
        "city_id",
    ),
    (
        module.DescribeAdvancedShapesRequest(
            selector=module.DescribeAdvancedShapesRequestSelectorCityDetailsVariant(
                city_details=module.ReportDetails(label="nested"),
            ),
        ),
        "city_details",
    ),
]
for request, field_name in cases:
    request_pb = module._to_pb_describe_advanced_shapes_request(request)
    assert request_pb.WhichOneof("selector") == field_name
    assert module._from_pb_describe_advanced_shapes_request(request_pb) == request
`)
}

func TestPythonMapper_RejectsOneofBaseVariant(t *testing.T) {
	runExamplePythonScript(t, `
request = module.DescribeAdvancedShapesRequest(
    selector=module.DescribeAdvancedShapesRequestSelectorVariant(),
)
try:
    module._to_pb_describe_advanced_shapes_request(request)
except TypeError as exc:
    assert "unsupported DescribeAdvancedShapesRequestSelectorVariant variant" in str(exc)
else:
    raise AssertionError("expected TypeError for base oneof wrapper")
`)
}

func TestPythonMapper_RoundTripsRecursiveMessages(t *testing.T) {
	runExamplePythonScript(t, `
request = module.DescribeAdvancedShapesRequest(
    tree=module.RecursiveNode(
        name="root",
        child=module.RecursiveNode(name="branch"),
        children=[
            module.RecursiveNode(
                name="child",
                children=[module.RecursiveNode(name="leaf")],
            )
        ],
    ),
)
request_pb = module._to_pb_describe_advanced_shapes_request(request)
assert request_pb.HasField("tree")
assert request_pb.tree.child.name == "branch"
assert request_pb.tree.children[0].children[0].name == "leaf"
assert module._from_pb_describe_advanced_shapes_request(request_pb) == request
`)
}

func TestPythonMapper_RoundTripsAnyAndValueFamilies(t *testing.T) {
	runExamplePythonScript(t, `
request = module.DescribeAdvancedShapesRequest(
    payload={"service": "example", "nested": {"ok": True}},
    items=[1, "two", {"three": 3}, None],
    dynamic={"kind": ["alpha", False, None]},
    detail_any={
        "@type": "type.googleapis.com/internal.testproto.example.v1.ReportDetails",
        "label": "from-any",
    },
    duration_any={
        "@type": "type.googleapis.com/google.protobuf.Duration",
        "value": "3600s",
    },
)
request_pb = module._to_pb_describe_advanced_shapes_request(request)
assert request_pb.HasField("payload")
assert request_pb.HasField("items")
assert request_pb.HasField("dynamic")
assert request_pb.HasField("detail_any")
assert request_pb.HasField("duration_any")
assert module._from_pb_describe_advanced_shapes_request(request_pb) == request
`)
}

func TestPythonMapper_RoundTripsSupportedWKTs(t *testing.T) {
	runExamplePythonScript(t, `
request = module.DescribeAdvancedShapesRequest(
    observed_at="2026-04-10T12:34:56Z",
    ttl="3600s",
    note="memo",
    total=42,
    enabled=True,
    ratio=3.5,
    mask="fooBar,bazQux",
    blob=b"\x00hi",
    small_total=-7,
    uint_total=9,
    huge_total=1234567890123,
    weight=1.25,
    raw_ratio=2.75,
)
request_pb = module._to_pb_describe_advanced_shapes_request(request)
assert request_pb.HasField("observed_at")
assert request_pb.HasField("ttl")
assert request_pb.HasField("note")
assert request_pb.HasField("total")
assert request_pb.HasField("enabled")
assert request_pb.HasField("ratio")
assert request_pb.HasField("mask")
assert request_pb.HasField("blob")
assert request_pb.HasField("small_total")
assert request_pb.HasField("uint_total")
assert request_pb.HasField("huge_total")
assert request_pb.HasField("weight")
assert request_pb.HasField("raw_ratio")
assert request_pb.uint_total.value == 9
assert request_pb.huge_total.value == 1234567890123
assert module._from_pb_describe_advanced_shapes_request(request_pb) == request

ping = module.PingResponse(ack=module.Empty())
ping_pb = module._to_pb_ping_response(ping)
assert ping_pb.HasField("ack")
assert module._from_pb_ping_response(ping_pb) == ping
`)
}

func TestPythonMapper_RejectsUnknownEnumNumbers(t *testing.T) {
	runExamplePythonScript(t, `
request_pb = example_pb2.DescribeScalarShapesRequest(
    bool_flag=True,
    text_value="hello",
    bytes_value=b"abc",
    int32_value=-1,
    sint32_value=-2,
    sfixed32_value=-3,
    uint32_value=7,
    fixed32_value=8,
    int64_value=-9,
    sint64_value=-10,
    sfixed64_value=-11,
    uint64_value=12,
    fixed64_value=13,
    float_value=1.25,
    double_value=2.5,
    status=123,
    details=example_pb2.ReportDetails(label="shape"),
    optional_status=456,
)

try:
    module._from_pb_describe_scalar_shapes_request(request_pb)
except ValueError as exc:
    assert "123" in str(exc)
else:
    raise AssertionError("expected ValueError for unknown enum number")
`)
}

func TestPythonRuntime_AcceptsExplicitNullForNonRequiredFields(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

async def handler(ctx, req):
    assert req.city == "Paris"
    assert req.count == 2
    assert req.details == module.ReportDetails(label="today")
    assert req.units is module.UNSET
    assert req.labels == ["daily"]
    return module.CreateReportResponse(
        report_id="report-1",
        total_count=2,
        status=module.ReportStatus.REPORT_STATUS_OK,
        details=req.details,
        warnings=[],
    )

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

result = asyncio.run(module._dispatch_call(
    registry,
    "example_CreateReport",
    {
        "city": "Paris",
        "count": 2,
        "details": {"label": "today"},
        "units": None,
        "labels": ["daily"],
    },
    None,
))

assert result.isError in (None, False)
assert result.structuredContent["reportId"] == "report-1"
assert result.structuredContent["totalCount"] == "2"
assert result.structuredContent["status"] == "REPORT_STATUS_OK"
assert result.structuredContent["details"] == {"label": "today"}
assert result.structuredContent["warnings"] == []
assert result.content[0].text
`)
}

func TestPythonRuntime_InvalidSchemaPayloadRaisesInvalidParams(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio
import mcp.shared.exceptions
import mcp.types

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, _req):
    raise AssertionError("handler must not run for schema-invalid input")

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

async def main():
    try:
        await module._dispatch_call(
            registry,
            "example_CreateReport",
            {
                "city": "Paris",
                "count": "two",
                "details": {"label": "today"},
            },
            None,
        )
    except mcp.shared.exceptions.McpError as exc:
        assert exc.error.code == mcp.types.INVALID_PARAMS
        assert "example_CreateReport" in exc.error.message
        assert "invalid arguments for tool" in exc.error.message
    else:
        raise AssertionError("expected McpError for schema validation failure")

asyncio.run(main())
`)
}

func TestPythonRuntime_InvalidProtoJSONPayloadRaisesInvalidParams(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio
import mcp.shared.exceptions
import mcp.types

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, _req):
    raise AssertionError("handler must not run for protojson-invalid input")

registry.add_tool(module._RegisteredTool(
    name="example_DescribeScalarShapes",
    title="Describe scalar shapes",
    description="Describe scalar protobuf kinds.",
    input_schema_json=module.EXAMPLE_API_DESCRIBE_SCALAR_SHAPES_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_DESCRIBE_SCALAR_SHAPES_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.DescribeScalarShapesRequest,
    response_type=example_pb2.DescribeScalarShapesResponse,
    from_pb=module._from_pb_describe_scalar_shapes_request,
    to_pb=module._to_pb_describe_scalar_shapes_response,
    handler=handler,
    annotations=None,
    icons=None,
))

invalid_arguments = {
    "boolFlag": True,
    "textValue": "hello",
    "bytesValue": "YWJj",
    "int32Value": -1,
    "sint32Value": -2,
    "sfixed32Value": -3,
    "uint32Value": 7,
    "fixed32Value": 8,
    "int64Value": "-9",
    "sint64Value": "-10",
    "sfixed64Value": "-11",
    "uint64Value": "12",
    "fixed64Value": "not-an-int",
    "floatValue": 1.25,
    "doubleValue": 2.5,
    "status": "REPORT_STATUS_OK",
    "details": {"label": "shape"},
}

async def main():
    try:
        await module._dispatch_call(
            registry,
            "example_DescribeScalarShapes",
            invalid_arguments,
            None,
        )
    except mcp.shared.exceptions.McpError as exc:
        assert exc.error.code == mcp.types.INVALID_PARAMS
        assert "example_DescribeScalarShapes" in exc.error.message
        assert "invalid arguments for tool" in exc.error.message
    else:
        raise AssertionError("expected McpError for protojson parse failure")

asyncio.run(main())
`)
}

func TestPythonRuntime_DispatchesSyncHandlers(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio
import json

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, req):
    assert req.city == "Paris"
    assert req.count == 2
    return module.CreateReportResponse(
        report_id="report-1",
        total_count=req.count,
        status=module.ReportStatus.REPORT_STATUS_OK,
        details=req.details,
        warnings=["cached"],
    )

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

result = asyncio.run(module._dispatch_call(
    registry,
    "example_CreateReport",
    {
        "city": "Paris",
        "count": 2,
        "details": {"label": "today"},
    },
    None,
))

assert result.isError in (None, False)
assert result.structuredContent["reportId"] == "report-1"
assert result.structuredContent["totalCount"] == "2"
assert result.structuredContent["status"] == "REPORT_STATUS_OK"
assert result.structuredContent["warnings"] == ["cached"]
assert json.loads(result.content[0].text) == result.structuredContent
`)
}

func TestPythonRuntime_RejectsUnknownEnumOutputAgainstSchema(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, req):
    return module.CreateReportResponse(
        report_id="report-1",
        total_count=req.count,
        status=123,
        details=req.details,
        warnings=[],
    )

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

try:
    asyncio.run(module._dispatch_call(
        registry,
        "example_CreateReport",
        {
            "city": "Paris",
            "count": 2,
            "details": {"label": "today"},
        },
        None,
    ))
except RuntimeError as exc:
    assert "mcpruntime: validate output for tool 'example_CreateReport'" in str(exc)
else:
    raise AssertionError("expected RuntimeError for unknown enum output")
`)
}

func TestPythonRuntime_HandlerBusinessErrorsReturnToolErrorResult(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, _req):
    raise RuntimeError("boom")

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

result = asyncio.run(module._dispatch_call(
    registry,
    "example_CreateReport",
    {
        "city": "Paris",
        "count": 2,
        "details": {"label": "today"},
    },
    None,
))

assert result.isError is True
assert len(result.content) == 1
assert result.content[0].text == "boom"
`)
}

func TestPythonRuntime_WrapsOutputMarshalFailures(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

async def handler(_ctx, _req):
    return object()

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._from_pb_create_report_request,
    to_pb=module._to_pb_create_report_response,
    handler=handler,
    annotations=None,
    icons=None,
))

try:
    asyncio.run(module._dispatch_call(
        registry,
        "example_CreateReport",
        {
            "city": "Paris",
            "count": 2,
            "details": {"label": "today"},
        },
        None,
    ))
except RuntimeError as exc:
    assert "mcpruntime: marshal output for tool 'example_CreateReport'" in str(exc)
else:
    raise AssertionError("expected RuntimeError for output marshal failure")
`)
}

func TestPythonRuntime_UnknownToolRaisesInvalidParams(t *testing.T) {
	runExamplePythonScript(t, `
import asyncio
import mcp.server.lowlevel
import mcp.shared.exceptions
import mcp.types

class Handler:
    def create_report(self, _ctx, req):
        return module.CreateReportResponse(
            report_id="report-1",
            total_count=req.count,
            status=module.ReportStatus.REPORT_STATUS_OK,
            details=req.details,
            warnings=[],
        )

    def ping(self, _ctx, _req):
        return module.PingResponse(ack=module.Empty())

    def describe_advanced_shapes(self, _ctx, req):
        return module.DescribeAdvancedShapesResponse(
            labels=req.labels,
            quantities=req.quantities,
            toggles=req.toggles,
            limits=req.limits,
            observed_at=req.observed_at,
            ttl=req.ttl,
            payload=req.payload,
            items=req.items,
            dynamic=req.dynamic,
            note=req.note,
            total=req.total,
            enabled=req.enabled,
            ratio=req.ratio,
            mask=req.mask,
            blob=req.blob,
            small_total=req.small_total,
            uint_total=req.uint_total,
            huge_total=req.huge_total,
            weight=req.weight,
            raw_ratio=req.raw_ratio,
            tree=req.tree,
            detail_any=req.detail_any,
            duration_any=req.duration_any,
            selector=req.selector,
        )

    def describe_scalar_shapes(self, _ctx, req):
        return module.DescribeScalarShapesResponse(
            bool_flag=req.bool_flag,
            text_value=req.text_value,
            bytes_value=req.bytes_value,
            int32_value=req.int32_value,
            sint32_value=req.sint32_value,
            sfixed32_value=req.sfixed32_value,
            uint32_value=req.uint32_value,
            fixed32_value=req.fixed32_value,
            int64_value=req.int64_value,
            sint64_value=req.sint64_value,
            sfixed64_value=req.sfixed64_value,
            uint64_value=req.uint64_value,
            fixed64_value=req.fixed64_value,
            float_value=req.float_value,
            double_value=req.double_value,
            status=req.status,
            details=req.details,
            samples=req.samples,
            optional_bool_flag=req.optional_bool_flag,
            optional_text_value=req.optional_text_value,
            optional_bytes_value=req.optional_bytes_value,
            optional_int32_value=req.optional_int32_value,
            optional_sint32_value=req.optional_sint32_value,
            optional_sfixed32_value=req.optional_sfixed32_value,
            optional_uint32_value=req.optional_uint32_value,
            optional_fixed32_value=req.optional_fixed32_value,
            optional_int64_value=req.optional_int64_value,
            optional_sint64_value=req.optional_sint64_value,
            optional_sfixed64_value=req.optional_sfixed64_value,
            optional_uint64_value=req.optional_uint64_value,
            optional_fixed64_value=req.optional_fixed64_value,
            optional_float_value=req.optional_float_value,
            optional_double_value=req.optional_double_value,
            optional_status=req.optional_status,
        )

    def hidden_thing(self, _ctx, _req):
        return module.HiddenThingResponse()

server = mcp.server.lowlevel.Server("mapper-test-server", version="1.0.0")
module.register_example_api_tools(server, Handler())

request = mcp.types.CallToolRequest(
    params=mcp.types.CallToolRequestParams(name="missing_tool", arguments={}),
)

async def main():
    try:
        await server.request_handlers[mcp.types.CallToolRequest](request)
    except mcp.shared.exceptions.McpError as exc:
        assert exc.error.code == mcp.types.INVALID_PARAMS
        assert "unknown tool" in exc.error.message
    else:
        raise AssertionError("expected McpError for unknown tool")

asyncio.run(main())
`)
}
