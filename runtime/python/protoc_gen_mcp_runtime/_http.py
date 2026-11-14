"""Streamable HTTP endpoint and OAuth resource server for generated servers.

Ports mcpruntime.NewConfiguredHTTPHandler and mcpruntime.NewOAuthResourceHandler
on top of the SDK: StreamableHTTPSessionManager serves MCP statelessly and
the SDK bearer middleware authenticates requests. This module adds what the
SDK does not provide: RFC 9728 metadata at the root and path-scoped URLs with
live ``scopes_supported``, and a scope check that reads the request body and
answers ``403 insufficient_scope`` before any tool runs.
"""

from __future__ import annotations

import json
import time
import urllib.parse
from dataclasses import dataclass, field
from typing import Any

import mcp.server.lowlevel
from mcp.server.auth.middleware.bearer_auth import BearerAuthBackend, RequireAuthMiddleware
from mcp.server.auth.provider import TokenVerifier
from mcp.server.streamable_http_manager import StreamableHTTPSessionManager
from starlette.middleware.authentication import AuthenticationMiddleware
from starlette.types import ASGIApp, Message, Receive, Scope, Send

from ._jwks import JWKSTokenVerifier
from ._tools import ToolRegistry, install_tool_registry

PROTECTED_RESOURCE_METADATA_PATH = "/.well-known/oauth-protected-resource"
MAX_REQUEST_BODY_BYTES = 4 << 20


@dataclass(frozen=True)
class OAuthResourceServer:
    """Public OAuth policy from ``option (mcp.options.v1.server).oauth``."""

    resource_url: str
    authorization_servers: tuple[str, ...]
    scopes: tuple[str, ...] = ()
    issuer: str = ""
    audience: str = ""
    jwks_url: str = ""
    mcp_path: str = ""
    metadata_path: str = field(init=False, default="")
    metadata_url: str = field(init=False, default="")

    def __post_init__(self) -> None:
        resource = urllib.parse.urlsplit(self.resource_url)
        if resource.scheme != "https" or not resource.netloc:
            raise ValueError(f"mcpruntime: invalid HTTPS resource URL {self.resource_url!r}")
        if not self.authorization_servers:
            raise ValueError("mcpruntime: at least one authorization server is required")
        for server in self.authorization_servers:
            parsed = urllib.parse.urlsplit(server)
            if parsed.scheme != "https" or not parsed.netloc:
                raise ValueError(f"mcpruntime: invalid HTTPS authorization server {server!r}")
        path = self.mcp_path or "/mcp"
        if not path.startswith("/") or path == "/" or path.startswith("/.well-known/"):
            raise ValueError(f"mcpruntime: invalid MCP path {path!r}")
        object.__setattr__(self, "mcp_path", path)
        metadata_path = PROTECTED_RESOURCE_METADATA_PATH + resource.path.rstrip("/")
        object.__setattr__(self, "metadata_path", metadata_path)
        object.__setattr__(self, "metadata_url", f"{resource.scheme}://{resource.netloc}{metadata_path}")


def new_http_app(
    server: mcp.server.lowlevel.Server,
    oauth: OAuthResourceServer | None = None,
    verifier: TokenVerifier | None = None,
) -> ASGIApp:
    """Serves server over stateless Streamable HTTP, protected when oauth is set.

    Returns an ASGI application for any ASGI server, for example uvicorn.
    Without oauth the endpoint is open on ``/mcp``. With oauth, ``verifier``
    validates bearer tokens; when it is None the built-in RS256 JWKS verifier
    is built from the oauth issuer, audience and JWKS URL.
    """
    if server is None:
        raise ValueError("mcpruntime: server is None")
    manager = StreamableHTTPSessionManager(app=server, stateless=True)

    async def mcp_endpoint(scope: Scope, receive: Receive, send: Send) -> None:
        await manager.handle_request(scope, receive, send)

    if oauth is None:
        return _HTTPApp(manager, {"/mcp": mcp_endpoint}, None)

    if verifier is None:
        if not oauth.jwks_url:
            raise ValueError("mcpruntime: OAuth needs a token verifier or a JWKS URL")
        verifier = JWKSTokenVerifier(jwks_url=oauth.jwks_url, issuer=oauth.issuer, audience=oauth.audience)
    registry = install_tool_registry(server)
    gate = _ScopeGate(mcp_endpoint, oauth, registry)
    protected: ASGIApp = RequireAuthMiddleware(gate, required_scopes=[], resource_metadata_url=oauth.metadata_url)
    protected = AuthenticationMiddleware(protected, backend=BearerAuthBackend(verifier))
    return _HTTPApp(manager, {oauth.mcp_path: protected}, _MetadataEndpoint(oauth, registry))


class _HTTPApp:
    """Routes the MCP endpoint and metadata, and runs the SDK session manager."""

    def __init__(
        self,
        manager: StreamableHTTPSessionManager,
        endpoints: dict[str, ASGIApp],
        metadata: _MetadataEndpoint | None,
    ) -> None:
        self.manager = manager
        self.endpoints = endpoints
        self.metadata = metadata

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] == "lifespan":
            await self._lifespan(receive, send)
            return
        path = scope.get("path", "")
        if self.metadata is not None and (
            path == PROTECTED_RESOURCE_METADATA_PATH or path.startswith(PROTECTED_RESOURCE_METADATA_PATH + "/")
        ):
            await self.metadata(scope, receive, send)
            return
        endpoint = self.endpoints.get(path)
        if endpoint is None:
            await _respond(send, 404, b"404 page not found\n")
            return
        if not _same_origin(scope):
            await _respond(send, 403, b"cross-origin request detected\n")
            return
        await endpoint(scope, receive, send)

    async def _lifespan(self, receive: Receive, send: Send) -> None:
        await receive()
        async with self.manager.run():
            await send({"type": "lifespan.startup.complete"})
            await receive()
        await send({"type": "lifespan.shutdown.complete"})


class _MetadataEndpoint:
    """RFC 9728 metadata whose scopes_supported includes every tool's scopes."""

    def __init__(self, oauth: OAuthResourceServer, registry: ToolRegistry) -> None:
        self.oauth = oauth
        self.registry = registry

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        path = scope.get("path", "")
        if path not in (PROTECTED_RESOURCE_METADATA_PATH, self.oauth.metadata_path):
            await _respond(send, 404, b"404 page not found\n")
            return
        cors = [
            (b"access-control-allow-origin", b"*"),
            (b"access-control-allow-methods", b"GET, OPTIONS"),
            (b"access-control-allow-headers", b"Content-Type"),
        ]
        method = scope.get("method", "GET")
        if method == "OPTIONS":
            await _respond(send, 204, b"", cors)
            return
        if method != "GET":
            await _respond(send, 405, b"Method not allowed\n", cors)
            return
        scopes = set(self.oauth.scopes)
        for tool in self.registry.tools.values():
            scopes.update(tool.required_scopes)
        document: dict[str, Any] = {"resource": self.oauth.resource_url}
        if self.oauth.authorization_servers:
            document["authorization_servers"] = list(self.oauth.authorization_servers)
        if scopes:
            document["scopes_supported"] = sorted(scopes)
        await _respond(send, 200, json.dumps(document).encode(), [*cors, (b"content-type", b"application/json")])


class _ScopeGate:
    """Requires global and per-tool scopes before a request reaches the SDK."""

    def __init__(self, app: ASGIApp, oauth: OAuthResourceServer, registry: ToolRegistry) -> None:
        self.app = app
        self.oauth = oauth
        self.registry = registry

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        user = scope.get("user")
        access_token = getattr(user, "access_token", None)
        if access_token is None or access_token.expires_at is None or access_token.expires_at <= time.time():
            await _respond(send, 401, b"invalid token\n", [(b"www-authenticate", f'Bearer resource_metadata="{self.oauth.metadata_url}"'.encode())])
            return
        required = set(self.oauth.scopes)
        if scope.get("method") == "POST":
            body = b""
            more = True
            while more:
                message = await receive()
                if message["type"] == "http.disconnect":
                    return
                body += message.get("body", b"")
                more = message.get("more_body", False)
                if len(body) > MAX_REQUEST_BODY_BYTES:
                    await _respond(send, 413, b"MCP request body too large\n")
                    return
            if body.strip():
                try:
                    required.update(self._tool_scopes(body))
                except ValueError:
                    await _respond(send, 400, b"invalid MCP request\n")
                    return
            receive = _replay(body, receive)
        missing = [name for name in sorted(required) if name not in access_token.scopes]
        if missing:
            challenge = (
                f'Bearer error="insufficient_scope", scope="{" ".join(sorted(required))}", '
                f'resource_metadata="{self.oauth.metadata_url}"'
            )
            await _respond(send, 403, b"insufficient scope\n", [(b"www-authenticate", challenge.encode()), (b"cache-control", b"no-store")])
            return
        await self.app(scope, receive, send)

    def _tool_scopes(self, body: bytes) -> set[str]:
        payload = json.loads(body)
        messages = payload if isinstance(payload, list) else [payload]
        scopes: set[str] = set()
        for message in messages:
            if not isinstance(message, dict) or message.get("method") != "tools/call" or "id" not in message:
                continue
            params = message.get("params")
            if not isinstance(params, dict):
                raise ValueError("tools/call params must be an object")
            name = params.get("name")
            if not isinstance(name, str):
                raise ValueError("tools/call name must be a string")
            tool = self.registry.tools.get(name)
            if tool is not None:
                scopes.update(tool.required_scopes)
        _reject_duplicate_names(body)
        return scopes


def _reject_duplicate_names(body: bytes) -> None:
    def pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
        keys = [key for key, _ in items]
        if keys.count("name") > 1:
            raise ValueError("ambiguous duplicate tool name")
        return dict(items)

    json.loads(body, object_pairs_hook=pairs)


def _replay(body: bytes, receive: Receive) -> Receive:
    """Delivers the buffered body once, then waits on the real connection."""
    sent = False

    async def replay() -> Message:
        nonlocal sent
        if sent:
            return await receive()
        sent = True
        return {"type": "http.request", "body": body, "more_body": False}

    return replay


def _same_origin(scope: Scope) -> bool:
    """Mirrors net/http CrossOriginProtection for state-changing requests."""
    if scope.get("method") in ("GET", "HEAD", "OPTIONS"):
        return True
    headers = {key.decode().lower(): value.decode() for key, value in scope.get("headers", [])}
    site = headers.get("sec-fetch-site")
    if site is not None:
        return site in ("same-origin", "none")
    origin = headers.get("origin")
    if origin is None:
        return True
    return urllib.parse.urlsplit(origin).netloc == headers.get("host", "")


async def _respond(send: Send, status: int, body: bytes, headers: list[tuple[bytes, bytes]] | None = None) -> None:
    response_headers = list(headers or [])
    if not any(key == b"content-type" for key, _ in response_headers):
        response_headers.append((b"content-type", b"text/plain; charset=utf-8"))
    response_headers.append((b"content-length", str(len(body)).encode()))
    await send({"type": "http.response.start", "status": status, "headers": response_headers})
    await send({"type": "http.response.body", "body": body})
