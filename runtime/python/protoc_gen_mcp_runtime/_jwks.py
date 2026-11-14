"""RS256 JWT verification against an HTTPS JWKS, ported from mcpruntime/jwks.go.

Implements the SDK ``TokenVerifier`` protocol. Every failure, including key
lookup and fetch errors, yields None so the SDK middleware answers 401.
"""

from __future__ import annotations

import asyncio
import base64
import time
import urllib.parse
from collections.abc import Callable
from typing import Any

import anyio
import httpx
import jwt
from jwt.algorithms import RSAAlgorithm
from mcp.server.auth.provider import AccessToken

JWKS_CACHE_TTL = 5 * 60
JWKS_REFRESH_TIMEOUT = 5.0
JWKS_UNKNOWN_KEY_INTERVAL = 60
JWKS_RETRY_MIN = 1
JWKS_RETRY_MAX = 60
JWKS_DEFAULT_MAX_STALE = 60 * 60
JWKS_MAX_BYTES = 1 << 20
LEEWAY_SECONDS = 10


class JWKSError(Exception):
    """A JWKS document or fetch could not be used."""


def _require_https(raw: str, what: str) -> None:
    parsed = urllib.parse.urlsplit(raw)
    if parsed.scheme != "https" or not parsed.hostname or parsed.fragment or parsed.username or parsed.password:
        raise ValueError(f"mcpruntime: {what} must be a valid HTTPS URL")


def _b64url_uint(value: str) -> int:
    if "=" in value:
        raise JWKSError("JWK values must be unpadded base64url")
    return int.from_bytes(base64.urlsafe_b64decode(value + "=" * (-len(value) % 4)), "big")


class JWKSTokenVerifier:
    """Verifies RS256 access tokens issued by issuer for audience.

    Keys come from jwks_url and are cached for five minutes; stale keys stay
    usable for ``max_stale`` seconds while refreshes fail (negative means the
    TTL only, zero the one-hour default), and unknown key ids refresh at most
    once a minute.
    """

    def __init__(
        self,
        *,
        jwks_url: str,
        issuer: str,
        audience: str,
        max_stale: float = 0,
        http_client: httpx.AsyncClient | None = None,
        clock: Callable[[], float] = time.time,
    ) -> None:
        _require_https(jwks_url, "JWKS URL")
        _require_https(issuer, "issuer")
        if not audience:
            raise ValueError("mcpruntime: JWT audience is required")
        self.jwks_url = jwks_url
        self.issuer = issuer
        self.audience = audience
        if max_stale < 0:
            self.trust_window = JWKS_CACHE_TTL
        elif max_stale == 0:
            self.trust_window = JWKS_DEFAULT_MAX_STALE
        else:
            self.trust_window = max(max_stale, JWKS_CACHE_TTL)
        self.http_client = http_client
        self.now = clock
        self.keys: dict[str, Any] = {}
        self.expires = 0.0
        self.hard_expires = 0.0
        self.retry_at = 0.0
        self.retry_delay = 0.0
        self.last_error: Exception | None = None
        self.last_unknown_refresh = float("-inf")
        self.refreshing: anyio.Event | None = None
        self.fetches = 0

    async def verify_token(self, token: str) -> AccessToken | None:
        try:
            header = jwt.get_unverified_header(token)
            kid = header.get("kid")
            if not isinstance(kid, str) or not kid:
                return None
            key = await self._key(kid)
            claims = jwt.decode(
                token,
                key,
                algorithms=["RS256"],
                audience=self.audience,
                issuer=self.issuer,
                leeway=LEEWAY_SECONDS,
                options={"require": ["exp", "iss", "aud"]},
            )
        except Exception:
            return None
        scopes = _scopes(claims)
        if scopes is None:
            return None
        return AccessToken(
            token=token,
            client_id=str(claims.get("client_id") or claims.get("azp") or claims.get("sub") or ""),
            scopes=scopes,
            expires_at=int(claims["exp"]),
            subject=claims.get("sub"),
            claims=claims,
        )

    async def _key(self, kid: str) -> Any:
        while True:
            now = self.now()
            key = self.keys.get(kid)
            if key is not None and now < self.hard_expires:
                if now >= self.expires and self.refreshing is None and now >= self.retry_at:
                    self._start_refresh(background=True)
                return key
            if self.refreshing is not None:
                await self.refreshing.wait()
                continue
            if now < self.retry_at and self.last_error is not None:
                raise self.last_error
            if key is None and now < self.expires:
                fresh = self.expires - now > JWKS_CACHE_TTL - JWKS_UNKNOWN_KEY_INTERVAL
                if fresh or now - self.last_unknown_refresh < JWKS_UNKNOWN_KEY_INTERVAL:
                    raise JWKSError("unknown JWT key id")
                self.last_unknown_refresh = now
            elif key is None:
                self.last_unknown_refresh = now
            await self._start_refresh(background=False)

    def _start_refresh(self, *, background: bool) -> Any:
        done = anyio.Event()
        self.refreshing = done

        async def run() -> None:
            try:
                keys = await self._fetch()
            except Exception as error:
                self.retry_delay = JWKS_RETRY_MIN if self.retry_delay == 0 else min(self.retry_delay * 2, JWKS_RETRY_MAX)
                self.retry_at = self.now() + self.retry_delay
                self.last_error = error
            else:
                now = self.now()
                self.keys = keys
                self.expires = now + JWKS_CACHE_TTL
                self.hard_expires = now + self.trust_window
                self.retry_delay = 0
                self.retry_at = 0
                self.last_error = None
            finally:
                self.refreshing = None
                done.set()

        if background:
            self._background = asyncio.get_running_loop().create_task(run())
            return None
        return run()

    async def _fetch(self) -> dict[str, Any]:
        self.fetches += 1
        client = self.http_client or httpx.AsyncClient(follow_redirects=True)
        try:
            with anyio.fail_after(JWKS_REFRESH_TIMEOUT):
                response = await client.get(self.jwks_url)
        finally:
            if self.http_client is None:
                await client.aclose()
        if response.url.scheme != "https":
            raise JWKSError("JWKS redirect to an insecure scheme is forbidden")
        if response.status_code != 200:
            raise JWKSError(f"JWKS endpoint returned HTTP {response.status_code}")
        if len(response.content) > JWKS_MAX_BYTES:
            raise JWKSError("JWKS response exceeds 1 MiB")
        document = response.json()
        keys: dict[str, Any] = {}
        for jwk in document.get("keys", []) if isinstance(document, dict) else []:
            key = _usable_key(jwk)
            if key is not None:
                keys[jwk["kid"]] = key
        if not keys:
            raise JWKSError("JWKS response contains no usable RS256 signing keys")
        return keys


def _usable_key(jwk: Any) -> Any:
    if not isinstance(jwk, dict) or not jwk.get("kid") or jwk.get("kty") != "RSA":
        return None
    if jwk.get("alg") not in (None, "", "RS256") or jwk.get("use") not in (None, "", "sig"):
        return None
    try:
        modulus = _b64url_uint(jwk["n"])
        exponent_bytes = base64.urlsafe_b64decode(jwk["e"] + "=" * (-len(jwk["e"]) % 4))
        if "=" in jwk["e"] or not 1 <= len(exponent_bytes) <= 4:
            return None
        exponent = int.from_bytes(exponent_bytes, "big")
        if exponent < 3 or exponent % 2 == 0 or modulus.bit_length() < 2048:
            return None
        return RSAAlgorithm.from_jwk(jwk)
    except Exception:
        return None


def _scopes(claims: dict[str, Any]) -> list[str] | None:
    scope = claims.get("scope")
    if isinstance(scope, str):
        return scope.split()
    scp = claims.get("scp")
    if isinstance(scp, list):
        if not all(isinstance(item, str) for item in scp):
            return None
        return list(scp)
    if isinstance(scp, str):
        return scp.split()
    return []
