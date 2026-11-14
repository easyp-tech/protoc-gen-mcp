"""Python counterpart of the Go HTTP fixture server for the shared HTTP contract."""

from __future__ import annotations

import socket
import sys
import time
from pathlib import Path

import anyio
import uvicorn
from mcp.server.auth.provider import AccessToken

_REPO_ROOT = Path(__file__).resolve().parents[2]
if str(_REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(_REPO_ROOT))

from internal.testproto.config.v1 import server_config_mcp

_DEMO_SCOPES = {
    "reader": ["reports:read"],
    "writer": ["reports:read", "reports:write"],
}


class DemoVerifier:
    async def verify_token(self, token: str) -> AccessToken | None:
        scopes = _DEMO_SCOPES.get(token)
        if scopes is None:
            return None
        return AccessToken(token=token, client_id="contract", scopes=scopes, expires_at=int(time.time()) + 3600)


class SecureHandler:
    def check_access(self, _ctx, _req: server_config_mcp.CheckAccessRequest) -> server_config_mcp.CheckAccessResponse:
        return server_config_mcp.CheckAccessResponse(allowed=True)


async def main() -> None:
    server = server_config_mcp.create_server_config_mcp_server(
        server_config_mcp.ServerConfigMCPHandlers(secure_api=SecureHandler())
    )
    app = server_config_mcp.create_server_config_mcp_http_app(server, DemoVerifier())
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    # Listen before announcing the address so clients never race uvicorn's startup.
    sock.listen(128)
    host, port = sock.getsockname()
    config = uvicorn.Config(app, log_level="warning", lifespan="on")
    http_server = uvicorn.Server(config)
    config.load()
    print(f"listening on http://{host}:{port}", flush=True)
    await http_server.serve(sockets=[sock])


if __name__ == "__main__":
    anyio.run(main)
