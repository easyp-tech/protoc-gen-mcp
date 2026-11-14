import Foundation
import JWTKit
import MCP
import ProtocGenMCPRuntime

#if canImport(FoundationNetworking)
    import FoundationNetworking
#endif

/// RS256 verifier ported from mcpruntime/jwks.go: HTTPS-only JWKS (redirects
/// included) with a five-minute cache, bounded staleness, unknown key ids
/// refreshed at most once a minute, exponential backoff and a single refresh
/// in flight. jwt-kit checks signatures and refuses RSA keys under 2048 bits.
public struct JWKSTokenVerifier: TokenVerifier {
    static let cacheTTL: TimeInterval = 5 * 60
    static let refreshTimeout: TimeInterval = 5
    static let unknownKeyInterval: TimeInterval = 60
    static let retryMin: TimeInterval = 1
    static let retryMax: TimeInterval = 60
    static let defaultMaxStale: TimeInterval = 60 * 60
    static let maxJWKSBytes = 1 << 20
    static let leeway: TimeInterval = 10

    /// Fetches the JWKS document; tests replace it. Returns the final URL
    /// after redirects, the status code and the body.
    public typealias Fetcher = @Sendable (URL) async throws -> (url: URL, status: Int, body: Data)

    public let jwksURL: URL
    public let issuer: String
    public let audience: String
    let cache: JWKSCache
    let clock: @Sendable () -> Date

    /// maxStale is how long a stale key set stays usable while refreshes
    /// fail: negative means the TTL only, zero one hour.
    public init(
        jwksURL: String,
        issuer: String,
        audience: String,
        maxStale: TimeInterval = 0,
        fetcher: Fetcher? = nil,
        clock: @escaping @Sendable () -> Date = Date.init
    ) throws {
        guard let url = Self.httpsURL(jwksURL) else {
            throw RuntimeError("mcpruntime: JWKS URL must be a valid HTTPS URL")
        }
        guard Self.httpsURL(issuer) != nil else {
            throw RuntimeError("mcpruntime: issuer must be a valid HTTPS URL")
        }
        if audience.isEmpty {
            throw RuntimeError("mcpruntime: JWT audience is required")
        }
        let trustWindow: TimeInterval
        if maxStale < 0 {
            trustWindow = Self.cacheTTL
        } else if maxStale == 0 {
            trustWindow = Self.defaultMaxStale
        } else {
            trustWindow = max(maxStale, Self.cacheTTL)
        }
        self.jwksURL = url
        self.issuer = issuer
        self.audience = audience
        self.clock = clock
        self.cache = JWKSCache(url: url, trustWindow: trustWindow, fetcher: fetcher ?? Self.fetchOverHTTPS, clock: clock)
    }

    static func httpsURL(_ raw: String) -> URL? {
        guard let components = URLComponents(string: raw), components.scheme == "https",
            let host = components.host, !host.isEmpty, components.fragment == nil,
            components.user == nil, components.password == nil
        else {
            return nil
        }
        return components.url
    }

    public func verify(_ token: String) async throws -> AccessToken? {
        guard let header = Self.header(of: token), header.alg == "RS256", let kid = header.kid, !kid.isEmpty else {
            return nil
        }
        let claims: Claims
        do {
            let key = try await cache.key(kid)
            let keys = JWTKeyCollection()
            await keys.add(rsa: key, digestAlgorithm: .sha256, kid: JWKIdentifier(string: kid))
            claims = try await keys.verify(token, as: Claims.self)
        } catch {
            return nil
        }
        let now = clock().timeIntervalSince1970
        guard claims.iss == issuer, claims.aud.contains(audience), let exp = claims.exp, now - Self.leeway < exp else {
            return nil
        }
        if let nbf = claims.nbf, now + Self.leeway < nbf {
            return nil
        }
        if let iat = claims.iat, now + Self.leeway < iat {
            return nil
        }
        guard let scopes = claims.scopes else {
            return nil
        }
        let subject = claims.sub ?? ""
        return AccessToken(
            token: token,
            clientID: claims.clientID ?? claims.azp ?? subject,
            subject: subject,
            scopes: scopes,
            expiresAt: Date(timeIntervalSince1970: exp)
        )
    }

    struct Header: Decodable {
        let alg: String?
        let kid: String?
    }

    static func header(of token: String) -> Header? {
        guard let encoded = token.split(separator: ".", omittingEmptySubsequences: false).first,
            let data = base64URLDecode(String(encoded))
        else {
            return nil
        }
        return try? JSONDecoder().decode(Header.self, from: data)
    }

    /// Registered claims checked by hand with the injected clock, plus scopes.
    struct Claims: JWTPayload {
        let iss: String?
        let aud: [String]
        let exp: Double?
        let nbf: Double?
        let iat: Double?
        let sub: String?
        let clientID: String?
        let azp: String?
        /// nil when scp holds something other than strings.
        let scopes: [String]?

        enum CodingKeys: String, CodingKey {
            case iss, aud, exp, nbf, iat, sub, azp, scope, scp
            case clientID = "client_id"
        }

        init(from decoder: Decoder) throws {
            let container = try decoder.container(keyedBy: CodingKeys.self)
            iss = try? container.decode(String.self, forKey: .iss)
            if let single = try? container.decode(String.self, forKey: .aud) {
                aud = [single]
            } else {
                aud = (try? container.decode([String].self, forKey: .aud)) ?? []
            }
            exp = try? container.decode(Double.self, forKey: .exp)
            nbf = try? container.decode(Double.self, forKey: .nbf)
            iat = try? container.decode(Double.self, forKey: .iat)
            sub = try? container.decode(String.self, forKey: .sub)
            clientID = try? container.decode(String.self, forKey: .clientID)
            azp = try? container.decode(String.self, forKey: .azp)
            if let scope = try? container.decode(String.self, forKey: .scope) {
                scopes = scope.split(whereSeparator: \.isWhitespace).map(String.init)
            } else if container.contains(.scp) {
                if let list = try? container.decode([String].self, forKey: .scp) {
                    scopes = list
                } else if let text = try? container.decode(String.self, forKey: .scp) {
                    scopes = text.split(whereSeparator: \.isWhitespace).map(String.init)
                } else {
                    scopes = nil
                }
            } else {
                scopes = []
            }
        }

        func encode(to encoder: Encoder) throws {}

        func verify(using algorithm: some JWTAlgorithm) async throws {}
    }

    static let fetchOverHTTPS: Fetcher = { url in
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = refreshTimeout
        configuration.timeoutIntervalForResource = refreshTimeout
        let session = URLSession(configuration: configuration, delegate: HTTPSOnlyRedirects(), delegateQueue: nil)
        defer { session.finishTasksAndInvalidate() }
        var request = URLRequest(url: url)
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        return try await withCheckedThrowingContinuation { continuation in
            session.dataTask(with: request) { data, response, error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                guard let http = response as? HTTPURLResponse else {
                    continuation.resume(throwing: RuntimeError("JWKS endpoint did not answer over HTTP"))
                    return
                }
                continuation.resume(returning: (http.url ?? url, http.statusCode, data ?? Data()))
            }.resume()
        }
    }
}

/// Refuses redirects that leave HTTPS, so the 3xx surfaces as an error.
private final class HTTPSOnlyRedirects: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
    func urlSession(
        _ session: URLSession,
        task: URLSessionTask,
        willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest,
        completionHandler: @escaping @Sendable (URLRequest?) -> Void
    ) {
        completionHandler(request.url?.scheme == "https" ? request : nil)
    }
}

/// Key cache shared by concurrent verifications of one verifier.
actor JWKSCache {
    private let url: URL
    private let trustWindow: TimeInterval
    private let fetcher: JWKSTokenVerifier.Fetcher
    private let clock: @Sendable () -> Date

    private var keys: [String: Insecure.RSA.PublicKey] = [:]
    private var expires = Date.distantPast
    private var hardExpires = Date.distantPast
    private(set) var retryAt = Date.distantPast
    private(set) var retryDelay: TimeInterval = 0
    private var lastError: Error?
    private var lastUnknownRefresh: Date?
    private var refreshing: Task<Void, Never>?
    private(set) var fetches = 0

    init(url: URL, trustWindow: TimeInterval, fetcher: @escaping JWKSTokenVerifier.Fetcher, clock: @escaping @Sendable () -> Date) {
        self.url = url
        self.trustWindow = trustWindow
        self.fetcher = fetcher
        self.clock = clock
    }

    func key(_ kid: String) async throws -> Insecure.RSA.PublicKey {
        while true {
            let now = clock()
            let key = keys[kid]
            if let key, now < hardExpires {
                if now >= expires, refreshing == nil, now >= retryAt {
                    startRefresh()
                }
                return key
            }
            if let refreshing {
                await refreshing.value
                continue
            }
            if now < retryAt, let lastError {
                throw lastError
            }
            if key == nil, now < expires {
                let fresh = expires.timeIntervalSince(now) > JWKSTokenVerifier.cacheTTL - JWKSTokenVerifier.unknownKeyInterval
                let throttled = lastUnknownRefresh.map { now.timeIntervalSince($0) < JWKSTokenVerifier.unknownKeyInterval } ?? false
                if fresh || throttled {
                    throw RuntimeError("unknown JWT key id")
                }
            }
            if key == nil {
                lastUnknownRefresh = now
            }
            startRefresh()
        }
    }

    private func startRefresh() {
        fetches += 1
        let fetcher = self.fetcher
        let url = self.url
        refreshing = Task {
            do {
                let fetched = try await withTimeout(JWKSTokenVerifier.refreshTimeout) {
                    try Self.usableKeys(try await fetcher(url))
                }
                self.finish(.success(fetched))
            } catch {
                self.finish(.failure(error))
            }
        }
    }

    private func finish(_ result: Result<[String: Insecure.RSA.PublicKey], Error>) {
        let now = clock()
        switch result {
        case .success(let fetched):
            keys = fetched
            expires = now.addingTimeInterval(JWKSTokenVerifier.cacheTTL)
            hardExpires = now.addingTimeInterval(trustWindow)
            retryDelay = 0
            retryAt = .distantPast
            lastError = nil
        case .failure(let error):
            retryDelay = retryDelay == 0 ? JWKSTokenVerifier.retryMin : min(retryDelay * 2, JWKSTokenVerifier.retryMax)
            retryAt = now.addingTimeInterval(retryDelay)
            lastError = error
        }
        refreshing = nil
    }

    static func usableKeys(_ response: (url: URL, status: Int, body: Data)) throws -> [String: Insecure.RSA.PublicKey] {
        if response.url.scheme != "https" {
            throw RuntimeError("JWKS redirect to an insecure scheme is forbidden")
        }
        if response.status != 200 {
            throw RuntimeError("JWKS endpoint returned HTTP \(response.status)")
        }
        if response.body.count > JWKSTokenVerifier.maxJWKSBytes {
            throw RuntimeError("JWKS response exceeds 1 MiB")
        }
        var keys: [String: Insecure.RSA.PublicKey] = [:]
        if case .object(let document) = try jsonValue(response.body), case .array(let entries)? = document["keys"] {
            for entry in entries {
                if let (kid, key) = usableKey(entry) {
                    keys[kid] = key
                }
            }
        }
        if keys.isEmpty {
            throw RuntimeError("JWKS response contains no usable RS256 signing keys")
        }
        return keys
    }

    static func usableKey(_ entry: Value) -> (String, Insecure.RSA.PublicKey)? {
        guard case .object(let jwk) = entry, case .string(let kid)? = jwk["kid"], !kid.isEmpty,
            case .string("RSA")? = jwk["kty"],
            case .string(let n)? = jwk["n"], case .string(let e)? = jwk["e"],
            !n.contains("="), !e.contains("=")
        else {
            return nil
        }
        for (field, allowed) in [("alg", "RS256"), ("use", "sig")] {
            switch jwk[field] {
            case nil, .string("")?:
                break
            case .string(let value)? where value == allowed:
                break
            default:
                return nil
            }
        }
        guard let exponent = base64URLDecode(e), (1...4).contains(exponent.count) else {
            return nil
        }
        let value = exponent.reduce(UInt64(0)) { $0 << 8 | UInt64($1) }
        guard value >= 3, value % 2 == 1, let key = try? Insecure.RSA.PublicKey(modulus: n, exponent: e) else {
            return nil
        }
        return (kid, key)
    }
}

func base64URLDecode(_ value: String) -> Data? {
    var base64 = value.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
    base64 += String(repeating: "=", count: (4 - base64.count % 4) % 4)
    return Data(base64Encoded: base64)
}

/// Bounds a whole refresh, body included, as http.Client.Timeout does in Go.
private func withTimeout<T: Sendable>(_ seconds: TimeInterval, _ operation: @escaping @Sendable () async throws -> T) async throws -> T {
    try await withThrowingTaskGroup(of: T.self) { group in
        group.addTask { try await operation() }
        group.addTask {
            try await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            throw RuntimeError("JWKS refresh timed out")
        }
        let result = try await group.next()!
        group.cancelAll()
        return result
    }
}
