from __future__ import annotations

import json
import os
from pathlib import Path
import sys

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

_REPO_ROOT = Path(__file__).resolve().parents[2]
if str(_REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(_REPO_ROOT))

from internal.testproto.example.v1 import example_mcp

_INVALID_OUTPUT_ENV = "PROTOC_GEN_MCP_PYTHON_INVALID_OUTPUT"


def _copy_report_details(value: example_mcp.ReportDetails) -> example_mcp.ReportDetails:
    return example_mcp.ReportDetails(label=value.label)


def _copy_recursive_node(
    value: example_mcp.RecursiveNode | example_mcp._UnsetType,
) -> example_mcp.RecursiveNode | example_mcp._UnsetType:
    if value is example_mcp.UNSET:
        return example_mcp.UNSET
    return example_mcp.RecursiveNode(
        name=value.name,
        child=_copy_recursive_node(value.child),
        children=[_copy_recursive_node(child) for child in value.children],
    )


def _copy_advanced_selector(
    value: example_mcp.DescribeAdvancedShapesRequestSelectorVariant | example_mcp._UnsetType,
) -> example_mcp.DescribeAdvancedShapesResponseSelectorVariant | example_mcp._UnsetType:
    if value is example_mcp.UNSET:
        return example_mcp.UNSET
    if isinstance(value, example_mcp.DescribeAdvancedShapesRequestSelectorCityAliasVariant):
        return example_mcp.DescribeAdvancedShapesResponseSelectorCityAliasVariant(
            city_alias=value.city_alias,
        )
    if isinstance(value, example_mcp.DescribeAdvancedShapesRequestSelectorCityIdVariant):
        return example_mcp.DescribeAdvancedShapesResponseSelectorCityIdVariant(
            city_id=value.city_id,
        )
    if isinstance(value, example_mcp.DescribeAdvancedShapesRequestSelectorCityDetailsVariant):
        return example_mcp.DescribeAdvancedShapesResponseSelectorCityDetailsVariant(
            city_details=_copy_report_details(value.city_details),
        )
    raise TypeError(f"unsupported selector variant: {type(value).__name__}")


class Handler:
    def create_report(
        self,
        _ctx: example_mcp.ToolRequestContext,
        req: example_mcp.CreateReportRequest,
    ) -> example_mcp.CreateReportResponse:
        return example_mcp.CreateReportResponse(
            report_id="report-1",
            total_count=42,
            status=example_mcp.ReportStatus.REPORT_STATUS_OK,
            details=_copy_report_details(req.details),
            warnings=["none"],
        )

    def ping(
        self,
        _ctx: example_mcp.ToolRequestContext,
        _req: example_mcp.PingRequest,
    ) -> example_mcp.PingResponse:
        return example_mcp.PingResponse(ack=example_mcp.Empty())

    def describe_advanced_shapes(
        self,
        _ctx: example_mcp.ToolRequestContext,
        req: example_mcp.DescribeAdvancedShapesRequest,
    ) -> example_mcp.DescribeAdvancedShapesResponse:
        return example_mcp.DescribeAdvancedShapesResponse(
            labels=dict(req.labels),
            quantities=dict(req.quantities),
            toggles=dict(req.toggles),
            limits=dict(req.limits),
            observed_at=req.observed_at,
            ttl=req.ttl,
            payload=req.payload,
            items=list(req.items) if req.items is not example_mcp.UNSET else example_mcp.UNSET,
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
            tree=_copy_recursive_node(req.tree),
            detail_any=dict(req.detail_any) if req.detail_any is not example_mcp.UNSET else example_mcp.UNSET,
            duration_any=dict(req.duration_any) if req.duration_any is not example_mcp.UNSET else example_mcp.UNSET,
            selector=_copy_advanced_selector(req.selector),
        )

    def hidden_thing(
        self,
        _ctx: example_mcp.ToolRequestContext,
        _req: example_mcp.HiddenThingRequest,
    ) -> example_mcp.HiddenThingResponse:
        return example_mcp.HiddenThingResponse()

    def describe_scalar_shapes(
        self,
        _ctx: example_mcp.ToolRequestContext,
        req: example_mcp.DescribeScalarShapesRequest,
    ) -> example_mcp.DescribeScalarShapesResponse:
        return example_mcp.DescribeScalarShapesResponse(
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
            details=_copy_report_details(req.details),
            samples=list(req.samples),
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


def _apply_invalid_output_mode() -> None:
    mode = os.getenv(_INVALID_OUTPUT_ENV)
    if mode in ("", None):
        return
    if mode != "create_report":
        raise ValueError(f"unsupported {_INVALID_OUTPUT_ENV} value: {mode}")

    example_mcp.EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON = json.dumps(
        {
            "type": "object",
            "properties": {
                "reportId": {"type": "integer"},
            },
            "required": ["reportId"],
            "additionalProperties": False,
        },
        separators=(",", ":"),
    )


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server(
        "protoc-gen-mcp-python-example-server",
        version="v0.0.1",
    )
    _apply_invalid_output_mode()
    example_mcp.register_example_api_tools(server, Handler())
    return server


async def run_stdio_server() -> None:
    server = new_server()
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )


def main() -> None:
    anyio.run(run_stdio_server)


if __name__ == "__main__":
    main()
