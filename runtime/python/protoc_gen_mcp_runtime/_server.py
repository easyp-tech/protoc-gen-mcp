"""Configured low-level servers built by generated server factories."""

from __future__ import annotations

from typing import Any

import mcp.server.lowlevel

APP_EXTENSION_ID = "io.modelcontextprotocol/ui"
APP_HTML_MIME_TYPE = "text/html;profile=mcp-app"


class ConfiguredServer(mcp.server.lowlevel.Server):
    """A low-level server that can advertise the MCP Apps extension.

    The SDK infers tools, prompts and resources capabilities from registered
    handlers; this subclass only adds ``capabilities.extensions``, which the
    typed model accepts as an extra field.
    """

    def __init__(self, name: str, version: str, *, apps: bool) -> None:
        super().__init__(name, version=version)
        self.apps = apps

    def get_capabilities(self, notification_options: Any, experimental_capabilities: dict[str, dict[str, Any]]) -> Any:
        capabilities = super().get_capabilities(notification_options, experimental_capabilities)
        if not self.apps:
            return capabilities
        extensions = dict(getattr(capabilities, "extensions", None) or {})
        extensions[APP_EXTENSION_ID] = {"mimeTypes": [APP_HTML_MIME_TYPE]}
        return capabilities.model_copy(update={"extensions": extensions})


def new_configured_server(name: str, version: str, *, apps: bool) -> ConfiguredServer:
    """Mirrors mcpruntime.NewConfiguredServer for generated factories."""
    if not name:
        raise ValueError("mcpruntime: configured server needs a name")
    return ConfiguredServer(name, version or "v0.0.1", apps=apps)
