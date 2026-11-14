import asyncio
import base64
import datetime
import http.server
import json
import ssl
import tempfile
import threading
import time
import unittest
from pathlib import Path

import httpx
import jwt
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

import protoc_gen_mcp_runtime as runtime

ISSUER = "https://issuer.example.com"
AUDIENCE = "https://mcp.example.com/mcp"


def _b64(number: int) -> str:
    raw = number.to_bytes((number.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


def _jwk(key: rsa.RSAPrivateKey, kid: str) -> dict:
    numbers = key.public_key().public_numbers()
    return {"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": _b64(numbers.n), "e": _b64(numbers.e)}


class _JWKSServer:
    """HTTPS server whose JWKS document, status and redirect tests can change."""

    def __init__(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")])
        now = datetime.datetime.now(datetime.timezone.utc)
        cert = (
            x509.CertificateBuilder()
            .subject_name(name)
            .issuer_name(name)
            .public_key(key.public_key())
            .serial_number(x509.random_serial_number())
            .not_valid_before(now - datetime.timedelta(minutes=1))
            .not_valid_after(now + datetime.timedelta(days=1))
            .add_extension(x509.SubjectAlternativeName([x509.DNSName("localhost")]), critical=False)
            .sign(key, hashes.SHA256())
        )
        self.cert_path = Path(self.directory.name, "cert.pem")
        key_path = Path(self.directory.name, "key.pem")
        self.cert_path.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
        key_path.write_bytes(
            key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption())
        )
        self.document: dict = {"keys": []}
        self.status = 200
        self.redirect: str | None = None
        self.requests = 0
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self) -> None:
                owner.requests += 1
                if owner.redirect is not None:
                    self.send_response(302)
                    self.send_header("Location", owner.redirect)
                    self.end_headers()
                    return
                body = json.dumps(owner.document).encode()
                self.send_response(owner.status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args) -> None:
                pass

        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.cert_path, key_path)
        self.httpd.socket = context.wrap_socket(self.httpd.socket, server_side=True)
        self.url = f"https://localhost:{self.httpd.server_address[1]}/jwks.json"
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()

    def close(self) -> None:
        self.httpd.shutdown()
        self.directory.cleanup()


class JWKSTokenVerifierTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.server = _JWKSServer()
        cls.key = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.server.close()

    def setUp(self) -> None:
        self.server.document = {"keys": [_jwk(self.key, "k1")]}
        self.server.status = 200
        self.server.redirect = None
        self.server.requests = 0
        self.clock = [time.time()]

    def _verifier(self, **kwargs) -> runtime.JWKSTokenVerifier:
        client = httpx.AsyncClient(verify=ssl.create_default_context(cafile=str(self.server.cert_path)), follow_redirects=True)
        return runtime.JWKSTokenVerifier(
            jwks_url=self.server.url,
            issuer=ISSUER,
            audience=AUDIENCE,
            http_client=client,
            clock=lambda: self.clock[0],
            **kwargs,
        )

    def _token(self, key=None, kid="k1", **claims) -> str:
        payload = {"iss": ISSUER, "aud": AUDIENCE, "sub": "user-123", "exp": int(time.time()) + 600, **claims}
        payload = {name: value for name, value in payload.items() if value is not None}
        return jwt.encode(payload, key or self.key, algorithm="RS256", headers={"kid": kid})

    def _verify(self, verifier, token):
        return asyncio.run(verifier.verify_token(token))

    def test_valid_token_yields_scopes_and_expiry(self) -> None:
        verifier = self._verifier()
        access = self._verify(verifier, self._token(scope="reports:read reports:write"))
        self.assertEqual(access.scopes, ["reports:read", "reports:write"])
        self.assertEqual(access.subject, "user-123")
        self.assertIsNotNone(access.expires_at)
        self.assertEqual(self._verify(verifier, self._token(scope=None, scp=["a", "b"])).scopes, ["a", "b"])

    def test_rejects_wrong_claims_and_keys(self) -> None:
        verifier = self._verifier()
        other = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        for token in (
            self._token(iss="https://evil.example.com"),
            self._token(aud="https://other.example.com"),
            self._token(exp=int(time.time()) - 600),
            self._token(exp=None),
            self._token(key=other),
            self._token(scp=[1]),
        ):
            with self.subTest(token=token[:20]):
                self.assertIsNone(self._verify(verifier, token))

    def test_rejects_invalid_configuration(self) -> None:
        for kwargs in (
            {"jwks_url": "http://issuer.example.com/jwks", "issuer": ISSUER, "audience": AUDIENCE},
            {"jwks_url": self.server.url, "issuer": "http://issuer.example.com", "audience": AUDIENCE},
            {"jwks_url": self.server.url, "issuer": ISSUER, "audience": ""},
        ):
            with self.subTest(kwargs=kwargs), self.assertRaises(ValueError):
                runtime.JWKSTokenVerifier(**kwargs)

    def test_rejects_unusable_documents(self) -> None:
        small = rsa.generate_private_key(public_exponent=65537, key_size=1024)
        for document in ({"keys": []}, {"keys": [_jwk(small, "k1")]}, {"keys": [{**_jwk(self.key, "k1"), "alg": "HS256"}]}):
            with self.subTest(document=str(document)[:40]):
                self.server.document = document
                self.assertIsNone(self._verify(self._verifier(), self._token()))
        self.server.document = {"keys": [_jwk(self.key, "k1")]}
        self.server.status = 502
        self.assertIsNone(self._verify(self._verifier(), self._token()))

    def test_refuses_redirect_to_http(self) -> None:
        self.server.redirect = "http://127.0.0.1:9/jwks.json"
        self.assertIsNone(self._verify(self._verifier(), self._token()))

    def test_unknown_kid_refresh_is_throttled(self) -> None:
        verifier = self._verifier()
        token = self._token()

        async def scenario():
            self.assertIsNotNone(await verifier.verify_token(token))
            self.clock[0] += 2 * 60
            for index in range(8):
                await verifier.verify_token(self._token(kid=f"unknown-{index}"))

        asyncio.run(scenario())
        self.assertEqual(self.server.requests, 2)

    def test_failed_refresh_backs_off_exponentially(self) -> None:
        self.server.status = 500
        verifier = self._verifier(max_stale=-1)
        token = self._token()

        async def scenario():
            delays = []
            for _ in range(8):
                await verifier.verify_token(token)
                delays.append(verifier.retry_delay)
                self.clock[0] = verifier.retry_at
            return delays

        self.assertEqual(asyncio.run(scenario()), [1, 2, 4, 8, 16, 32, 60, 60])


class StdioScopeGuardTest(unittest.TestCase):
    def test_scoped_tool_without_token_is_denied(self) -> None:
        import mcp.server.lowlevel
        from google.protobuf import struct_pb2

        server = mcp.server.lowlevel.Server("guard")
        registry = runtime.install_tool_registry(server)
        registry.add_tool(
            runtime.RegisteredTool(
                name="secure",
                title="secure",
                description="",
                input_schema_json='{"type":"object"}',
                output_schema_json='{"type":"object"}',
                request_type=struct_pb2.Struct,
                response_type=struct_pb2.Struct,
                from_pb=runtime.identity,
                to_pb=runtime.identity,
                handler=lambda _ctx, request: request,
                annotations=None,
                icons=None,
                required_scopes=("reports:write",),
            )
        )
        denied = asyncio.run(runtime.dispatch_call(registry, "secure", {}, None))
        self.assertEqual(denied.content[0].text, "permission denied: an authenticated token is required")
        missing = asyncio.run(runtime.dispatch_call(registry, "secure", {}, None, ["reports:read"]))
        self.assertEqual(missing.content[0].text, 'permission denied: missing required scope "reports:write"')
        allowed = asyncio.run(runtime.dispatch_call(registry, "secure", {}, None, ["reports:write"]))
        self.assertFalse(allowed.isError)


if __name__ == "__main__":
    unittest.main()
