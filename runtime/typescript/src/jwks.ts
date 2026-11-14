import { InvalidTokenError } from "@modelcontextprotocol/sdk/server/auth/errors.js";
import type { AuthInfo } from "@modelcontextprotocol/sdk/server/auth/types.js";
import { decodeProtectedHeader, importJWK, jwtVerify } from "jose";
import type { CryptoKey, JWK, KeyObject } from "jose";

const CACHE_TTL_MS = 5 * 60_000;
const REFRESH_TIMEOUT_MS = 5_000;
const UNKNOWN_KEY_INTERVAL_MS = 60_000;
const RETRY_MIN_MS = 1_000;
const RETRY_MAX_MS = 60_000;
const DEFAULT_MAX_STALE_MS = 60 * 60_000;
const MAX_JWKS_BYTES = 1 << 20;
const LEEWAY_SECONDS = 10;

type VerificationKey = CryptoKey | KeyObject | Uint8Array;

export type JWKSVerifierConfig = {
  jwksUrl: string;
  issuer: string;
  audience: string;
  /** Seconds a stale key set stays usable while refreshes fail; negative means the TTL only, zero one hour. */
  maxStaleSeconds?: number;
  fetch?: typeof fetch;
  now?: () => number;
};

function requireHttps(raw: string, what: string): void {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`mcpruntime: ${what} must be a valid HTTPS URL`);
  }
  if (url.protocol !== "https:" || url.hostname === "" || url.hash !== "" || url.username !== "" || url.password !== "") {
    throw new Error(`mcpruntime: ${what} must be a valid HTTPS URL`);
  }
}

function base64UrlBytes(value: unknown): Uint8Array | undefined {
  if (typeof value !== "string" || value.includes("=") || !/^[A-Za-z0-9_-]*$/.test(value)) {
    return undefined;
  }
  return new Uint8Array(Buffer.from(value, "base64url"));
}

/**
 * RS256 verifier ported from mcpruntime/jwks.go: HTTPS-only JWKS with a
 * five-minute cache, bounded staleness, throttled unknown-kid refreshes and
 * exponential backoff. Every failure is an InvalidTokenError, so the SDK
 * bearer middleware answers 401.
 */
/**
 * Validates bearer tokens for the HTTP handler. It is structurally the SDK
 * OAuthTokenVerifier, declared here so consumers need no express typings.
 * Throw InvalidTokenError for a token that must get 401.
 */
export type TokenVerifier = {
  verifyAccessToken(token: string): Promise<AuthInfo>;
};

export class JWKSTokenVerifier implements TokenVerifier {
  readonly jwksUrl: string;
  readonly issuer: string;
  readonly audience: string;
  private readonly trustWindowMs: number;
  private readonly fetchImpl: typeof fetch;
  private readonly now: () => number;
  private keys = new Map<string, VerificationKey>();
  private expires = 0;
  private hardExpires = 0;
  retryAt = 0;
  retryDelayMs = 0;
  private lastError: Error | undefined;
  private lastUnknownRefresh = Number.NEGATIVE_INFINITY;
  private refreshing: Promise<void> | undefined;
  fetches = 0;

  constructor(config: JWKSVerifierConfig) {
    requireHttps(config.jwksUrl, "JWKS URL");
    requireHttps(config.issuer, "issuer");
    if (config.audience === "") {
      throw new Error("mcpruntime: JWT audience is required");
    }
    this.jwksUrl = config.jwksUrl;
    this.issuer = config.issuer;
    this.audience = config.audience;
    const maxStale = (config.maxStaleSeconds ?? 0) * 1000;
    this.trustWindowMs = maxStale < 0 ? CACHE_TTL_MS : maxStale === 0 ? DEFAULT_MAX_STALE_MS : Math.max(maxStale, CACHE_TTL_MS);
    this.fetchImpl = config.fetch ?? fetch;
    this.now = config.now ?? Date.now;
  }

  async verifyAccessToken(token: string): Promise<AuthInfo> {
    try {
      const header = decodeProtectedHeader(token);
      if (typeof header.kid !== "string" || header.kid === "") {
        throw new Error("missing JWT key id");
      }
      const key = await this.key(header.kid);
      const { payload } = await jwtVerify(token, key, {
        algorithms: ["RS256"],
        issuer: this.issuer,
        audience: this.audience,
        clockTolerance: LEEWAY_SECONDS,
        currentDate: new Date(this.now()),
        requiredClaims: ["exp", "iss", "aud"],
      });
      const scopes = scopesOf(payload);
      return {
        token,
        clientId: String(payload.client_id ?? payload.azp ?? payload.sub ?? ""),
        scopes,
        expiresAt: payload.exp,
        extra: { sub: payload.sub },
      };
    } catch (error) {
      throw new InvalidTokenError(error instanceof Error ? error.message : String(error));
    }
  }

  private async key(kid: string): Promise<VerificationKey> {
    for (;;) {
      const now = this.now();
      const key = this.keys.get(kid);
      if (key !== undefined && now < this.hardExpires) {
        if (now >= this.expires && this.refreshing === undefined && now >= this.retryAt) {
          void this.refresh();
        }
        return key;
      }
      if (this.refreshing !== undefined) {
        await this.refreshing;
        continue;
      }
      if (now < this.retryAt && this.lastError !== undefined) {
        throw this.lastError;
      }
      if (key === undefined && now < this.expires) {
        const fresh = this.expires - now > CACHE_TTL_MS - UNKNOWN_KEY_INTERVAL_MS;
        if (fresh || now - this.lastUnknownRefresh < UNKNOWN_KEY_INTERVAL_MS) {
          throw new Error("unknown JWT key id");
        }
      }
      if (key === undefined) {
        this.lastUnknownRefresh = now;
      }
      await this.refresh();
    }
  }

  private refresh(): Promise<void> {
    if (this.refreshing !== undefined) {
      return this.refreshing;
    }
    this.refreshing = (async () => {
      try {
        const keys = await this.fetchKeys();
        const now = this.now();
        this.keys = keys;
        this.expires = now + CACHE_TTL_MS;
        this.hardExpires = now + this.trustWindowMs;
        this.retryDelayMs = 0;
        this.retryAt = 0;
        this.lastError = undefined;
      } catch (error) {
        this.retryDelayMs = this.retryDelayMs === 0 ? RETRY_MIN_MS : Math.min(this.retryDelayMs * 2, RETRY_MAX_MS);
        this.retryAt = this.now() + this.retryDelayMs;
        this.lastError = error instanceof Error ? error : new Error(String(error));
      } finally {
        this.refreshing = undefined;
      }
    })();
    return this.refreshing;
  }

  private async fetchKeys(): Promise<Map<string, VerificationKey>> {
    this.fetches += 1;
    const response = await this.fetchImpl(this.jwksUrl, { signal: AbortSignal.timeout(REFRESH_TIMEOUT_MS), redirect: "follow" });
    if (response.url !== "" && new URL(response.url).protocol !== "https:") {
      throw new Error("JWKS redirect to an insecure scheme is forbidden");
    }
    if (response.status !== 200) {
      throw new Error(`JWKS endpoint returned HTTP ${response.status}`);
    }
    const body = await response.text();
    if (Buffer.byteLength(body) > MAX_JWKS_BYTES) {
      throw new Error("JWKS response exceeds 1 MiB");
    }
    const document = JSON.parse(body) as { keys?: unknown };
    const keys = new Map<string, VerificationKey>();
    for (const jwk of Array.isArray(document.keys) ? document.keys : []) {
      const key = await usableKey(jwk);
      if (key !== undefined) {
        keys.set((jwk as JWK).kid as string, key);
      }
    }
    if (keys.size === 0) {
      throw new Error("JWKS response contains no usable RS256 signing keys");
    }
    return keys;
  }
}

async function usableKey(raw: unknown): Promise<VerificationKey | undefined> {
  if (raw === null || typeof raw !== "object") {
    return undefined;
  }
  const jwk = raw as JWK;
  if (typeof jwk.kid !== "string" || jwk.kid === "" || jwk.kty !== "RSA") {
    return undefined;
  }
  if (!(jwk.alg === undefined || jwk.alg === "" || jwk.alg === "RS256") || !(jwk.use === undefined || jwk.use === "" || jwk.use === "sig")) {
    return undefined;
  }
  const modulus = base64UrlBytes(jwk.n);
  const exponent = base64UrlBytes(jwk.e);
  if (modulus === undefined || exponent === undefined || exponent.length < 1 || exponent.length > 4) {
    return undefined;
  }
  let e = 0;
  for (const byte of exponent) {
    e = e * 256 + byte;
  }
  let bits = modulus.length * 8;
  for (let index = 0; index < modulus.length && modulus[index] === 0; index++) {
    bits -= 8;
  }
  const first = modulus.find((byte) => byte !== 0) ?? 0;
  bits -= Math.clz32(first) - 24;
  if (e < 3 || e % 2 === 0 || bits < 2048) {
    return undefined;
  }
  try {
    return await importJWK({ kty: "RSA", n: jwk.n, e: jwk.e }, "RS256");
  } catch {
    return undefined;
  }
}

function scopesOf(payload: Record<string, unknown>): string[] {
  if (typeof payload.scope === "string") {
    return payload.scope.split(/\s+/).filter((scope) => scope !== "");
  }
  if (Array.isArray(payload.scp)) {
    if (!payload.scp.every((scope) => typeof scope === "string")) {
      throw new Error("scp claim must contain strings");
    }
    return [...payload.scp] as string[];
  }
  if (typeof payload.scp === "string") {
    return payload.scp.split(/\s+/).filter((scope) => scope !== "");
  }
  return [];
}
