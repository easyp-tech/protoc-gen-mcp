import Foundation
import JWTKit
import MCP
import XCTest
import _CryptoExtras

@testable import ProtocGenMCPHTTP

final class JWKSTokenVerifierTests: XCTestCase {
    static let issuer = "https://issuer.example.com"
    static let audience = "https://mcp.example.com/mcp"
    static let jwksURL = URL(string: "https://issuer.example.com/jwks.json")!

    struct TestClaims: JWTPayload {
        var iss: String? = JWKSTokenVerifierTests.issuer
        var aud: String? = JWKSTokenVerifierTests.audience
        var sub: String? = "user-123"
        var exp: Double? = Date().timeIntervalSince1970 + 600
        var nbf: Double?
        var iat: Double?
        var scope: String?
        var scp: [Int]?

        func verify(using algorithm: some JWTAlgorithm) async throws {}
    }

    /// A JWKS endpoint the tests reconfigure, counting requests.
    final class FakeEndpoint: @unchecked Sendable {
        private let lock = NSLock()
        var document = Data()
        var status = 200
        var finalURL = JWKSTokenVerifierTests.jwksURL
        private(set) var requests = 0

        var fetcher: JWKSTokenVerifier.Fetcher {
            { _ in
                self.lock.withLock {
                    self.requests += 1
                    return (self.finalURL, self.status, self.document)
                }
            }
        }
    }

    final class Clock: @unchecked Sendable {
        private let lock = NSLock()
        private var current = Date()
        var now: Date {
            get { lock.lock(); defer { lock.unlock() }; return current }
            set { lock.lock(); current = newValue; lock.unlock() }
        }
    }

    static func jwk(_ key: _RSA.Signing.PublicKey, kid: String, extra: [String: String] = [:]) throws -> [String: String] {
        let primitives = try key.getKeyPrimitives()
        var jwk = ["kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64URL(primitives.modulus), "e": base64URL(primitives.publicExponent)]
        jwk.merge(extra) { _, new in new }
        return jwk
    }

    static func base64URL(_ data: Data) -> String {
        data.base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }

    static func document(_ keys: [[String: String]]) throws -> Data {
        try JSONSerialization.data(withJSONObject: ["keys": keys])
    }

    var signingKey: _RSA.Signing.PrivateKey!
    var endpoint: FakeEndpoint!
    var clock: Clock!

    override func setUp() async throws {
        signingKey = try _RSA.Signing.PrivateKey(keySize: .bits2048)
        endpoint = FakeEndpoint()
        endpoint.document = try Self.document([Self.jwk(signingKey.publicKey, kid: "k1")])
        clock = Clock()
    }

    func verifier(maxStale: TimeInterval = 0) throws -> JWKSTokenVerifier {
        let clock = self.clock!
        return try JWKSTokenVerifier(
            jwksURL: Self.jwksURL.absoluteString,
            issuer: Self.issuer,
            audience: Self.audience,
            maxStale: maxStale,
            fetcher: endpoint.fetcher,
            clock: { clock.now }
        )
    }

    func token(_ claims: TestClaims = TestClaims(), key: _RSA.Signing.PrivateKey? = nil, kid: String? = "k1") async throws -> String {
        let keys = JWTKeyCollection()
        let signer = try JWTKit.Insecure.RSA.PrivateKey(pem: (key ?? signingKey).pemRepresentation)
        await keys.add(rsa: signer, digestAlgorithm: .sha256, kid: kid.map { JWKIdentifier(string: $0) })
        return try await keys.sign(claims, kid: kid.map { JWKIdentifier(string: $0) })
    }

    func testValidTokenYieldsScopesAndExpiry() async throws {
        let verifier = try self.verifier()
        var claims = TestClaims()
        claims.scope = "reports:read reports:write"
        let access = try await verifier.verify(try await token(claims))
        XCTAssertEqual(access?.scopes, ["reports:read", "reports:write"])
        XCTAssertEqual(access?.subject, "user-123")
        XCTAssertNotNil(access?.expiresAt)
    }

    func testRejectsWrongClaimsAndKeys() async throws {
        let verifier = try self.verifier()
        let now = Date().timeIntervalSince1970
        var cases: [TestClaims] = []
        var wrongIssuer = TestClaims(); wrongIssuer.iss = "https://evil.example.com"; cases.append(wrongIssuer)
        var wrongAudience = TestClaims(); wrongAudience.aud = "https://other.example.com"; cases.append(wrongAudience)
        var expired = TestClaims(); expired.exp = now - 600; cases.append(expired)
        var noExpiry = TestClaims(); noExpiry.exp = nil; cases.append(noExpiry)
        var futureIssued = TestClaims(); futureIssued.iat = now + 600; cases.append(futureIssued)
        var notYetValid = TestClaims(); notYetValid.nbf = now + 600; cases.append(notYetValid)
        var badScp = TestClaims(); badScp.scp = [1]; cases.append(badScp)
        for claims in cases {
            let rejected = try await verifier.verify(try await token(claims))
            XCTAssertNil(rejected, "\(claims)")
        }
        let other = try _RSA.Signing.PrivateKey(keySize: .bits2048)
        let wrongKey = try await verifier.verify(try await token(key: other))
        XCTAssertNil(wrongKey)
        let noKid = try await verifier.verify(try await token(kid: nil))
        XCTAssertNil(noKid)
        let garbage = try await verifier.verify("not-a-jwt")
        XCTAssertNil(garbage)
    }

    func testRejectsInvalidConfiguration() {
        for (url, issuer, audience) in [
            ("http://issuer.example.com/jwks", Self.issuer, Self.audience),
            (Self.jwksURL.absoluteString, "http://issuer.example.com", Self.audience),
            (Self.jwksURL.absoluteString, Self.issuer, ""),
            ("https://user:pass@issuer.example.com/jwks", Self.issuer, Self.audience),
        ] {
            XCTAssertThrowsError(try JWKSTokenVerifier(jwksURL: url, issuer: issuer, audience: audience))
        }
    }

    func testRejectsUnusableDocuments() async throws {
        let small = try _RSA.Signing.PrivateKey(unsafeKeySize: .init(bitCount: 1024))
        let documents = [
            try Self.document([]),
            try Self.document([Self.jwk(small.publicKey, kid: "k1")]),
            try Self.document([Self.jwk(signingKey.publicKey, kid: "k1", extra: ["alg": "HS256"])]),
            try Self.document([Self.jwk(signingKey.publicKey, kid: "k1", extra: ["use": "enc"])]),
            Data(#"{"keys":[]} trailing"#.utf8),
        ]
        for document in documents {
            endpoint.document = document
            let rejected = try await verifier().verify(try await token())
            XCTAssertNil(rejected)
        }
        endpoint.document = try Self.document([Self.jwk(signingKey.publicKey, kid: "k1")])
        endpoint.status = 502
        let failedFetch = try await verifier().verify(try await token())
        XCTAssertNil(failedFetch)
        endpoint.status = 200
        endpoint.finalURL = URL(string: "http://issuer.example.com/jwks.json")!
        let insecure = try await verifier().verify(try await token())
        XCTAssertNil(insecure)
    }

    func testUnknownKidRefreshIsThrottled() async throws {
        let verifier = try self.verifier()
        let first = try await verifier.verify(try await token())
        XCTAssertNotNil(first)
        clock.now = clock.now.addingTimeInterval(120)
        for index in 0..<8 {
            _ = try await verifier.verify(try await token(kid: "unknown-\(index)"))
        }
        XCTAssertEqual(endpoint.requests, 2)
    }

    func testStaleKeysExpireAfterTrustWindow() async throws {
        let verifier = try self.verifier(maxStale: 600)
        let token = try await self.token()
        let fresh = try await verifier.verify(token)
        XCTAssertNotNil(fresh)
        endpoint.status = 500
        clock.now = clock.now.addingTimeInterval(540)
        let stale = try await verifier.verify(token)
        XCTAssertNotNil(stale)
        clock.now = clock.now.addingTimeInterval(120)
        // Let the background refresh started above record its failure.
        try await Task.sleep(nanoseconds: 50_000_000)
        let expired = try await verifier.verify(token)
        XCTAssertNil(expired)
    }

    func testFailedRefreshBacksOffExponentially() async throws {
        endpoint.status = 500
        let verifier = try self.verifier(maxStale: -1)
        let token = try await self.token()
        var delays: [TimeInterval] = []
        for _ in 0..<8 {
            _ = try await verifier.verify(token)
            delays.append(await verifier.cache.retryDelay)
            clock.now = await verifier.cache.retryAt
        }
        XCTAssertEqual(delays, [1, 2, 4, 8, 16, 32, 60, 60])
        XCTAssertEqual(endpoint.requests, 8)
    }
}
