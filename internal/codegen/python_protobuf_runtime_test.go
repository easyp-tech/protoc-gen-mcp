package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
)

func renderExamplePythonForProtobufRuntimeTests(t *testing.T) string {
	t.Helper()

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

	return string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.py"))
}

func repoRootForPythonProtobufRuntimeTests(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func prepareExamplePythonProtobufRuntime(t *testing.T) (tempRoot string, repoRoot string) {
	t.Helper()

	repoRoot = repoRootForPythonProtobufRuntimeTests(t)
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
	if err := os.WriteFile(generatedPath, []byte(renderExamplePythonForProtobufRuntimeTests(t)), 0o644); err != nil {
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

func runExamplePythonProtobufScript(t *testing.T, script string) {
	t.Helper()

	tempRoot, repoRoot := prepareExamplePythonProtobufRuntime(t)
	cmd := pythontest.Command(t, "-c", fmt.Sprintf(
		"from internal.testproto.example.v1 import example_mcp as module\n"+
			"from internal.testproto.example.v1 import example_pb2\n%s",
		script,
	))
	cmd.Dir = tempRoot
	cmd.Env = pythontest.Env(t, "PYTHONPATH="+tempRoot+string(os.PathListSeparator)+repoRoot)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python protobuf runtime failed: %v\n%s", err, output)
	}
}

func TestPythonProtobufRuntime_RegistersToolsThroughLowLevelServer(t *testing.T) {
	runExamplePythonProtobufScript(t, `
import asyncio
import json
import mcp.server.lowlevel
import mcp.server.lowlevel.server as lowlevel_server
import mcp.types
from mcp.shared.context import RequestContext

seen_requests = []

class Handler:
    def create_report(self, _ctx, req):
        assert isinstance(req, example_pb2.CreateReportRequest)
        seen_requests.append(req)
        return example_pb2.CreateReportResponse(
            report_id="pb-1",
            total_count=req.count,
            status=example_pb2.REPORT_STATUS_OK,
            details=req.details,
            warnings=[],
        )

    def ping(self, _ctx, _req):
        return example_pb2.PingResponse()

    def describe_advanced_shapes(self, _ctx, req):
        return example_pb2.DescribeAdvancedShapesResponse(
            labels=req.labels,
            quantities=req.quantities,
            toggles=req.toggles,
            limits=req.limits,
        )

    def describe_scalar_shapes(self, _ctx, req):
        return example_pb2.DescribeScalarShapesResponse(
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
        )

    def hidden_thing(self, _ctx, _req):
        return example_pb2.HiddenThingResponse()

server = mcp.server.lowlevel.Server("protobuf-runtime-test", version="1.0.0")
module.register_example_api_tools(server, Handler(), namespace="raw.pb")

async def main():
    tools_result = await server.request_handlers[mcp.types.ListToolsRequest](mcp.types.ListToolsRequest())
    tool_names = [tool.name for tool in tools_result.root.tools]
    assert "raw_pb_CreateReport" in tool_names
    assert "raw_pb_Health" in tool_names
    assert all("." not in name for name in tool_names)

    token = lowlevel_server.request_ctx.set(RequestContext("test", None, object(), None))
    try:
        result = await server.request_handlers[mcp.types.CallToolRequest](
            mcp.types.CallToolRequest(
                params=mcp.types.CallToolRequestParams(
                    name="raw_pb_CreateReport",
                    arguments={"city": "Paris", "count": 2, "details": {"label": "today"}},
                ),
            ),
        )
    finally:
        lowlevel_server.request_ctx.reset(token)

    assert len(seen_requests) == 1
    assert result.root.structuredContent["reportId"] == "pb-1"
    assert result.root.structuredContent["totalCount"] == "2"
    assert result.root.structuredContent["status"] == "REPORT_STATUS_OK"
    assert json.loads(result.root.content[0].text) == result.root.structuredContent

asyncio.run(main())
`)
}

func TestPythonProtobufRuntime_DispatchesRawPB2Handlers(t *testing.T) {
	runExamplePythonProtobufScript(t, `
import asyncio
import json

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())
seen_requests = []

def handler(_ctx, req):
    assert isinstance(req, example_pb2.CreateReportRequest)
    seen_requests.append(req)
    return example_pb2.CreateReportResponse(
        report_id="pb-raw",
        total_count=req.count,
        status=example_pb2.REPORT_STATUS_OK,
        details=req.details,
        warnings=["protobuf"],
    )

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._identity,
    to_pb=module._identity,
    handler=handler,
    annotations=None,
    icons=None,
))

result = asyncio.run(module._dispatch_call(
    registry,
    "example_CreateReport",
    {"city": "Paris", "count": 2, "details": {"label": "today"}},
    None,
))

assert len(seen_requests) == 1
assert isinstance(seen_requests[0], example_pb2.CreateReportRequest)
assert result.isError in (None, False)
assert result.structuredContent["reportId"] == "pb-raw"
assert result.structuredContent["totalCount"] == "2"
assert result.structuredContent["status"] == "REPORT_STATUS_OK"
assert result.structuredContent["warnings"] == ["protobuf"]
assert json.loads(result.content[0].text) == result.structuredContent
`)
}

func TestPythonProtobufRuntime_InvalidSchemaPayloadRaisesInvalidParams(t *testing.T) {
	runExamplePythonProtobufScript(t, `
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
    from_pb=module._identity,
    to_pb=module._identity,
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

func TestPythonProtobufRuntime_InvalidProtoJSONPayloadRaisesInvalidParams(t *testing.T) {
	runExamplePythonProtobufScript(t, `
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
    from_pb=module._identity,
    to_pb=module._identity,
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

func TestPythonProtobufRuntime_RejectsInvalidOutput(t *testing.T) {
	runExamplePythonProtobufScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

def handler(_ctx, req):
    return example_pb2.CreateReportResponse(
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
    from_pb=module._identity,
    to_pb=module._identity,
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

func TestPythonProtobufRuntime_WrapsOutputMarshalFailures(t *testing.T) {
	runExamplePythonProtobufScript(t, `
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
    from_pb=module._identity,
    to_pb=module._identity,
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

func TestPythonProtobufRuntime_DispatchesAsyncHandlers(t *testing.T) {
	runExamplePythonProtobufScript(t, `
import asyncio

class DummyServer:
    pass

registry = module._ServerToolRegistry(DummyServer())

async def handler(_ctx, req):
    await asyncio.sleep(0)
    assert isinstance(req, example_pb2.CreateReportRequest)
    return example_pb2.CreateReportResponse(
        report_id="async-42",
        total_count=req.count,
        status=example_pb2.REPORT_STATUS_OK,
        details=req.details,
    )

registry.add_tool(module._RegisteredTool(
    name="example_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._identity,
    to_pb=module._identity,
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
assert result.structuredContent["reportId"] == "async-42"
assert result.structuredContent["totalCount"] == "2"
assert result.structuredContent["status"] == "REPORT_STATUS_OK"
`)
}

func TestPythonProtobufRuntime_HandlerBusinessErrorsReturnToolErrorResult(t *testing.T) {
	runExamplePythonProtobufScript(t, `
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
    from_pb=module._identity,
    to_pb=module._identity,
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
assert result.content[0].type == "text"
assert result.content[0].text == "boom"
assert result.structuredContent is None
`)
}

func TestPythonProtobufRuntime_UnknownToolRaisesInvalidParams(t *testing.T) {
	runExamplePythonProtobufScript(t, `
import asyncio
import mcp.types
from mcp.server.lowlevel import Server
from mcp.shared.exceptions import McpError

class Handler(module.ExampleAPIToolHandler):
    def create_report(self, _ctx, _req):
        return example_pb2.CreateReportResponse(
            report_id="ok",
            status=example_pb2.REPORT_STATUS_OK,
            details=example_pb2.ReportDetails(label="ok"),
        )

    def ping(self, _ctx, _req):
        return example_pb2.PingResponse()

    def describe_advanced_shapes(self, _ctx, _req):
        return example_pb2.DescribeAdvancedShapesResponse()

    def describe_scalar_shapes(self, _ctx, _req):
        return example_pb2.DescribeScalarShapesResponse()

    def hidden_thing(self, _ctx, _req):
        return example_pb2.HiddenThingResponse()

async def main():
    server = Server("protobuf-runtime-test", version="1.0.0")
    module.register_example_api_tools(server, Handler())
    registry = module._get_registry(server)

    try:
        await registry.call_tool("missing_tool", {"city": "Paris", "count": 2})
    except McpError as exc:
        assert exc.error.code == mcp.types.INVALID_PARAMS
        assert "missing_tool" in exc.error.message
        assert "unknown tool" in exc.error.message
    else:
        raise AssertionError("expected missing tool to raise McpError")

asyncio.run(main())
`)
}

func TestPythonProtobufRuntime_DuplicateRegistrationFails(t *testing.T) {
	runExamplePythonProtobufScript(t, `
from mcp.server.lowlevel import Server

class Handler(module.ExampleAPIToolHandler):
    def create_report(self, _ctx, _req):
        return example_pb2.CreateReportResponse(
            report_id="ok",
            status=example_pb2.REPORT_STATUS_OK,
            details=example_pb2.ReportDetails(label="ok"),
        )

    def ping(self, _ctx, _req):
        return example_pb2.PingResponse()

    def describe_advanced_shapes(self, _ctx, _req):
        return example_pb2.DescribeAdvancedShapesResponse()

    def describe_scalar_shapes(self, _ctx, _req):
        return example_pb2.DescribeScalarShapesResponse()

    def hidden_thing(self, _ctx, _req):
        return example_pb2.HiddenThingResponse()

server = Server("protobuf-runtime-test", version="1.0.0")
handler = Handler()
module.register_example_api_tools(server, handler)

try:
    module.register_example_api_tools(server, handler)
except ValueError as exc:
    assert "duplicate tool registration: example_CreateReport" in str(exc)
else:
    raise AssertionError("expected duplicate registration to fail")
`)
}

func TestPythonProtobufRuntime_NamespaceAndMetadataParity(t *testing.T) {
	runExamplePythonProtobufScript(t, `
expected_icons = [{
    "src": "https://example.com/method.png",
    "mimeType": "image/png",
    "sizes": ["32x32"],
    "theme": "dark",
}]

tool = module._build_tool(module._RegisteredTool(
    name="raw_pb_CreateReport",
    title="Create report",
    description="Create a report for a city.",
    input_schema_json=module.EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,
    output_schema_json=module.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,
    request_type=example_pb2.CreateReportRequest,
    response_type=example_pb2.CreateReportResponse,
    from_pb=module._identity,
    to_pb=module._identity,
    handler=lambda _ctx, _req: example_pb2.CreateReportResponse(),
    annotations={
        "readOnlyHint": True,
        "destructiveHint": False,
        "idempotentHint": True,
        "openWorldHint": False,
    },
    icons=expected_icons,
    execution={"taskSupport": "required"},
))

assert tool.name == "raw_pb_CreateReport"
annotations = tool.annotations.model_dump(by_alias=True, exclude_none=True)
assert annotations["readOnlyHint"] is True
assert annotations["destructiveHint"] is False
assert annotations["idempotentHint"] is True
assert annotations["openWorldHint"] is False
icons = [icon.model_dump(by_alias=True, exclude_none=True) for icon in tool.icons]
assert icons == expected_icons
execution = tool.execution.model_dump(by_alias=True, exclude_none=True)
assert execution["taskSupport"] == "required"
`)
}
