"""Helpers shared by the tool, prompt and resource registries."""

from __future__ import annotations

import inspect
from typing import Any

from google.protobuf import json_format


async def maybe_await(result: Any) -> Any:
    """Awaits result when a handler returned an awaitable."""
    if inspect.isawaitable(result):
        return await result
    return result


def identity(value: Any) -> Any:
    """Mapper used when handlers already work with protobuf messages."""
    return value


def message_to_json(message: Any) -> str:
    """Encodes a message as ProtoJSON, emitting fields without presence."""
    kwargs = {"preserving_proto_field_name": False}
    try:
        return json_format.MessageToJson(message, always_print_fields_with_no_presence=True, **kwargs)
    except TypeError:
        return json_format.MessageToJson(message, including_default_value_fields=True, **kwargs)
