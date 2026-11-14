import Foundation
import MCP
import ProtocGenMCPRuntime

/// A framework-neutral HTTP response, like the SDK's HTTPRequest on the way in:
/// adapters for Hummingbird, Vapor or SwiftNIO copy it to the wire.
public struct MCPHTTPResponse: Sendable {
    public enum Body: Sendable {
        case data(Data)
        case stream(AsyncThrowingStream<Data, Swift.Error>)
    }

    public var status: Int
    public var headers: [String: String]
    public var body: Body

    public init(status: Int, headers: [String: String] = [:], body: Body = .data(Data())) {
        self.status = status
        self.headers = headers
        self.body = body
    }

    init(_ response: HTTPResponse) {
        var headers = response.headers
        switch response {
        case .stream(let stream, _):
            self.init(status: response.statusCode, headers: headers, body: .stream(stream))
        default:
            let data = response.bodyData ?? Data()
            if !data.isEmpty, headers.keys.first(where: { $0.lowercased() == "content-type" }) == nil {
                headers["Content-Type"] = "application/json"
            }
            self.init(status: response.statusCode, headers: headers, body: .data(data))
        }
    }

    static func text(_ status: Int, _ message: String, headers: [String: String] = [:]) -> MCPHTTPResponse {
        var all = headers
        all["Content-Type"] = "text/plain; charset=utf-8"
        all["X-Content-Type-Options"] = "nosniff"
        return MCPHTTPResponse(status: status, headers: all, body: .data(Data(message.utf8)))
    }
}

/// Serves generated servers over stateless Streamable HTTP, ported from
/// mcpruntime.NewConfiguredHTTPHandler and NewOAuthResourceHandler. Each
/// request gets a fresh server on the SDK StatelessHTTPServerTransport. With
/// an OAuth policy it adds RFC 9728 metadata with live scopes_supported,
/// bearer authentication and a per-tool scope check that answers 403
/// insufficient_scope before any tool runs. Route every path to ``handle(_:)``.
public struct MCPHTTPHandler: Sendable {
    /// The request body limit of the Go SDK; adapters should stop reading past it.
    public static let maxRequestBodyBytes = 4 << 20

    private let oauth: OAuthResourceServer?
    private let verifier: (any TokenVerifier)?
    private let factory: @Sendable () async throws -> Server
    private let probe: ScopeProbe

    public init(
        oauth: OAuthResourceServer?,
        verifier: (any TokenVerifier)?,
        factory: @escaping @Sendable () async throws -> Server
    ) throws {
        self.oauth = oauth
        if let oauth {
            self.verifier = try verifier ?? oauth.jwksVerifier()
        } else {
            self.verifier = nil
        }
        self.factory = factory
        self.probe = ScopeProbe(factory: factory)
    }

    public var mcpPath: String {
        oauth?.mcpPath ?? "/mcp"
    }

    public func handle(_ request: HTTPRequest) async -> MCPHTTPResponse {
        let path = Self.path(of: request)
        if let oauth, path == OAuthResourceServer.metadataPrefix || path.hasPrefix(OAuthResourceServer.metadataPrefix + "/") {
            return await metadata(request, path: path, oauth: oauth)
        }
        guard path == mcpPath else {
            return .text(404, "404 page not found\n")
        }
        guard Self.sameOrigin(request) else {
            return .text(403, "cross-origin request detected\n")
        }
        var token: AccessToken?
        if let oauth {
            token = await authenticate(request)
            if token == nil {
                return .text(401, "invalid token\n", headers: ["WWW-Authenticate": oauth.bearerChallenge])
            }
        }
        guard request.method.uppercased() == "POST" else {
            if let oauth, let token, !Set(oauth.scopes).isSubset(of: token.scopes) {
                return insufficientScope(oauth, Set(oauth.scopes))
            }
            return .text(405, "Method not allowed.\n", headers: ["Allow": "POST"])
        }
        let body = request.body ?? Data()
        if body.count > Self.maxRequestBodyBytes {
            return .text(413, "MCP request body too large\n")
        }
        if let oauth, let token {
            var required = Set(oauth.scopes)
            do {
                required.formUnion(try requestedToolScopes(body, toolScopes: try await probe.toolScopes()))
            } catch is StrictJSON.Failure {
                return .text(400, "invalid MCP request\n")
            } catch {
                return .text(500, "mcpruntime: \(error)\n")
            }
            if !required.isSubset(of: token.scopes) {
                return insufficientScope(oauth, required)
            }
        }
        do {
            let server = try await factory()
            registry(for: server).accessToken = token
            let transport = StatelessHTTPServerTransport(
                validationPipeline: StandardValidationPipeline(validators: [
                    AcceptHeaderValidator(mode: .jsonOnly),
                    ContentTypeValidator(),
                    ProtocolVersionValidator(),
                ])
            )
            try await serve(server, transport: transport)
            let response = await transport.handleRequest(request)
            await server.stop()
            return MCPHTTPResponse(response)
        } catch {
            return .text(500, "mcpruntime: \(error)\n")
        }
    }

    private func authenticate(_ request: HTTPRequest) async -> AccessToken? {
        guard let verifier, let header = request.header(HTTPHeaderName.authorization), header.count > 7,
            header.prefix(7).lowercased() == "bearer "
        else {
            return nil
        }
        let raw = header.dropFirst(7).trimmingCharacters(in: .whitespaces)
        guard !raw.isEmpty, let token = try? await verifier.verify(raw), let expiresAt = token.expiresAt, expiresAt > Date() else {
            return nil
        }
        return token
    }

    private func insufficientScope(_ oauth: OAuthResourceServer, _ required: Set<String>) -> MCPHTTPResponse {
        .text(403, "insufficient scope\n", headers: [
            "WWW-Authenticate": oauth.insufficientScopeChallenge(required),
            "Cache-Control": "no-store",
        ])
    }

    private func metadata(_ request: HTTPRequest, path: String, oauth: OAuthResourceServer) async -> MCPHTTPResponse {
        guard path == OAuthResourceServer.metadataPrefix || path == oauth.metadataPath else {
            return .text(404, "404 page not found\n")
        }
        let cors = [
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type",
        ]
        switch request.method.uppercased() {
        case "OPTIONS":
            return MCPHTTPResponse(status: 204, headers: cors)
        case "GET":
            let scopes = (try? await probe.toolScopes().values.flatMap { $0 }) ?? []
            var headers = cors
            headers["Content-Type"] = "application/json"
            return MCPHTTPResponse(status: 200, headers: headers, body: .data(oauth.metadataJSON(toolScopes: scopes)))
        default:
            return .text(405, "Method not allowed\n", headers: cors)
        }
    }

    static func path(of request: HTTPRequest) -> String {
        let raw = request.path ?? "/"
        return String(raw.split(separator: "?", maxSplits: 1, omittingEmptySubsequences: false).first ?? "/")
    }

    /// Mirrors net/http CrossOriginProtection for state-changing requests.
    static func sameOrigin(_ request: HTTPRequest) -> Bool {
        if ["GET", "HEAD", "OPTIONS"].contains(request.method.uppercased()) {
            return true
        }
        if let site = request.header("Sec-Fetch-Site") {
            return site == "same-origin" || site == "none"
        }
        guard let origin = request.header(HTTPHeaderName.origin) else {
            return true
        }
        guard let components = URLComponents(string: origin), let host = components.host else {
            return false
        }
        let authority = components.port.map { "\(host):\($0)" } ?? host
        return authority == request.header(HTTPHeaderName.host)
    }
}

/// Builds one server lazily to learn which scopes each tool requires; tools
/// do not change between requests.
private actor ScopeProbe {
    private let factory: @Sendable () async throws -> Server
    private var scopes: [String: [String]]?

    init(factory: @escaping @Sendable () async throws -> Server) {
        self.factory = factory
    }

    func toolScopes() async throws -> [String: [String]] {
        if let scopes {
            return scopes
        }
        let probed = ProtocGenMCPRuntime.toolScopes(try await factory())
        scopes = probed
        return probed
    }
}
