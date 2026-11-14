import assert from "node:assert/strict";
import { generateKeyPairSync } from "node:crypto";
import { test } from "node:test";

import { InvalidTokenError } from "@modelcontextprotocol/sdk/server/auth/errors.js";
import { SignJWT, exportJWK, generateKeyPair } from "jose";

import { JWKSTokenVerifier } from "../dist/index.js";

const ISSUER = "https://issuer.example.com";
const AUDIENCE = "https://mcp.example.com/mcp";
const JWKS_URL = "https://issuer.example.com/jwks.json";

const { privateKey, publicKey } = await generateKeyPair("RS256", { modulusLength: 2048, extractable: true });
const otherKey = (await generateKeyPair("RS256", { modulusLength: 2048 })).privateKey;
const smallJwk = generateKeyPairSync("rsa", { modulusLength: 1024 }).publicKey.export({ format: "jwk" });
const jwk = { ...(await exportJWK(publicKey)), kid: "k1", alg: "RS256", use: "sig" };

function jwksServer() {
  const server = { document: { keys: [jwk] }, status: 200, url: JWKS_URL, requests: 0 };
  server.fetch = async () => {
    server.requests += 1;
    return { status: server.status, url: server.url, text: async () => JSON.stringify(server.document) };
  };
  return server;
}

function verifier(server, clock, extra = {}) {
  return new JWKSTokenVerifier({ jwksUrl: JWKS_URL, issuer: ISSUER, audience: AUDIENCE, fetch: server.fetch, now: () => clock.now, ...extra });
}

async function token({ key = privateKey, kid = "k1", claims = {}, exp = Math.floor(Date.now() / 1000) + 600 } = {}) {
  let jwt = new SignJWT({ scope: "reports:read reports:write", ...claims })
    .setProtectedHeader({ alg: "RS256", kid })
    .setIssuer(claims.iss ?? ISSUER)
    .setAudience(claims.aud ?? AUDIENCE)
    .setSubject("user-123");
  if (exp !== null) {
    jwt = jwt.setExpirationTime(exp);
  }
  return jwt.sign(key);
}

test("valid tokens yield scopes and expiry", async () => {
  const clock = { now: Date.now() };
  const auth = await verifier(jwksServer(), clock).verifyAccessToken(await token());
  assert.deepEqual(auth.scopes, ["reports:read", "reports:write"]);
  assert.equal(typeof auth.expiresAt, "number");
});

test("wrong claims, keys and documents are invalid tokens", async () => {
  const clock = { now: Date.now() };
  const server = jwksServer();
  const cases = [
    await token({ claims: { iss: "https://evil.example.com" } }),
    await token({ claims: { aud: "https://other.example.com" } }),
    await token({ exp: Math.floor(Date.now() / 1000) - 600 }),
    await token({ exp: null }),
    await token({ key: otherKey }),
  ];
  for (const candidate of cases) {
    await assert.rejects(verifier(server, clock).verifyAccessToken(candidate), InvalidTokenError);
  }
  for (const document of [{ keys: [] }, { keys: [{ ...smallJwk, kid: "k1" }] }, { keys: [{ ...jwk, alg: "HS256" }] }]) {
    const bad = jwksServer();
    bad.document = document;
    await assert.rejects(verifier(bad, clock).verifyAccessToken(await token()), InvalidTokenError);
  }
  const redirected = jwksServer();
  redirected.url = "http://issuer.example.com/jwks.json";
  await assert.rejects(verifier(redirected, clock).verifyAccessToken(await token()), InvalidTokenError);
});

test("configuration must use HTTPS and name an audience", () => {
  assert.throws(() => new JWKSTokenVerifier({ jwksUrl: "http://issuer.example.com/jwks", issuer: ISSUER, audience: AUDIENCE }));
  assert.throws(() => new JWKSTokenVerifier({ jwksUrl: JWKS_URL, issuer: "http://issuer.example.com", audience: AUDIENCE }));
  assert.throws(() => new JWKSTokenVerifier({ jwksUrl: JWKS_URL, issuer: ISSUER, audience: "" }));
});

test("unknown key ids refresh at most once a minute", async () => {
  const clock = { now: Date.now() };
  const server = jwksServer();
  const subject = verifier(server, clock);
  await subject.verifyAccessToken(await token());
  clock.now += 2 * 60_000;
  for (let index = 0; index < 8; index++) {
    await assert.rejects(subject.verifyAccessToken(await token({ kid: `unknown-${index}` })));
  }
  assert.equal(server.requests, 2);
});

test("failed refreshes back off exponentially", async () => {
  const clock = { now: Date.now() };
  const server = jwksServer();
  server.status = 500;
  const subject = verifier(server, clock, { maxStaleSeconds: -1 });
  const delays = [];
  for (let index = 0; index < 8; index++) {
    await assert.rejects(subject.verifyAccessToken(await token()));
    delays.push(subject.retryDelayMs);
    clock.now = subject.retryAt;
  }
  assert.deepEqual(delays, [1000, 2000, 4000, 8000, 16000, 32000, 60000, 60000]);
});
