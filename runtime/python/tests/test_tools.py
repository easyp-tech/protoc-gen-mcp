import asyncio
import json
import unittest

import mcp.server.lowlevel
import mcp.shared.exceptions
import mcp.types
from google.protobuf import struct_pb2

import protoc_gen_mcp_runtime as runtime

_OBJECT_SCHEMA = json.dumps({"type": "object"})


def _tool(name, handler, output_schema=_OBJECT_SCHEMA):
    return runtime.RegisteredTool(
        name=name,
        title=name,
        description="",
        input_schema_json=_OBJECT_SCHEMA,
        output_schema_json=output_schema,
        request_type=struct_pb2.Struct,
        response_type=struct_pb2.Struct,
        from_pb=runtime.identity,
        to_pb=runtime.identity,
        handler=handler,
        annotations=None,
        icons=None,
    )


def _echo(_ctx, request):
    return request


def _list(server):
    return asyncio.run(server.request_handlers[mcp.types.ListToolsRequest](mcp.types.ListToolsRequest()))


class ToolRegistryTest(unittest.TestCase):
    def test_names_are_unique_per_server_across_modules(self):
        server = mcp.server.lowlevel.Server("tools")
        runtime.install_tool_registry(server).add_tool(_tool("a_echo", _echo))
        with self.assertRaisesRegex(ValueError, "duplicate tool registration: a_echo"):
            runtime.install_tool_registry(server).add_tool(_tool("a_echo", _echo))
        other = mcp.server.lowlevel.Server("other")
        runtime.install_tool_registry(other).add_tool(_tool("a_echo", _echo))

    def test_composes_with_existing_low_level_handlers(self):
        server = mcp.server.lowlevel.Server("tools")

        @server.list_tools()
        async def _list_tools():
            return [mcp.types.Tool(name="manual", inputSchema={"type": "object"})]

        runtime.install_tool_registry(server).add_tool(_tool("generated", _echo))
        names = [tool.name for tool in _list(server).root.tools]
        self.assertEqual(names, ["manual", "generated"])

    def test_merged_duplicate_with_existing_handler_fails(self):
        server = mcp.server.lowlevel.Server("tools")

        @server.list_tools()
        async def _list_tools():
            return [mcp.types.Tool(name="clash", inputSchema={"type": "object"})]

        runtime.install_tool_registry(server).add_tool(_tool("clash", _echo))
        with self.assertRaisesRegex(ValueError, "duplicate tool registration: clash"):
            _list(server)

    def test_dispatch_validates_and_wraps_output(self):
        server = mcp.server.lowlevel.Server("tools")
        registry = runtime.install_tool_registry(server)
        registry.add_tool(_tool("echo", _echo))
        registry.add_tool(_tool("strict", _echo, output_schema=json.dumps({"type": "object", "required": ["id"]})))
        registry.add_tool(_tool("boom", lambda _ctx, _req: (_ for _ in ()).throw(RuntimeError("business failure"))))

        result = asyncio.run(runtime.dispatch_call(registry, "echo", {"a": 1}, None))
        self.assertEqual(result.structuredContent, {"a": 1})
        self.assertEqual(json.loads(result.content[0].text), {"a": 1})

        with self.assertRaisesRegex(RuntimeError, "mcpruntime: validate output for tool 'strict'"):
            asyncio.run(runtime.dispatch_call(registry, "strict", {}, None))

        failed = asyncio.run(runtime.dispatch_call(registry, "boom", {}, None))
        self.assertTrue(failed.isError)
        self.assertEqual(failed.content[0].text, "business failure")

        with self.assertRaises(mcp.shared.exceptions.McpError) as unknown:
            asyncio.run(runtime.dispatch_call(registry, "missing", {}, None))
        self.assertEqual(unknown.exception.error.code, mcp.types.INVALID_PARAMS)

    def test_tool_names_normalize_namespaces(self):
        self.assertEqual(runtime.tool_name("raw.pb", "Create"), "raw_pb_Create")
        self.assertEqual(runtime.tool_name("", "Create"), "Create")
        self.assertEqual(runtime.normalize_namespace(None, "default.ns"), "default_ns")


class GencodeVersionTest(unittest.TestCase):
    def test_mismatch_fails_import(self):
        runtime.check_gencode_version(runtime.GENCODE_VERSION)
        with self.assertRaisesRegex(ImportError, "regenerate the code"):
            runtime.check_gencode_version(runtime.GENCODE_VERSION + 1)


if __name__ == "__main__":
    unittest.main()
