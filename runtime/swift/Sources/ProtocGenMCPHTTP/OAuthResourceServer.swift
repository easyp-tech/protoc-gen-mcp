import Foundation
import ProtocGenMCPRuntime

/// Validates bearer tokens for the protected endpoint, like auth.TokenVerifier
/// in the Go runtime. Returning nil or throwing rejects the token with 401.
public protocol TokenVerifier: Sendable {
    func verify(_ token: String) async throws -> AccessToken?
}

/// Public OAuth policy from `option (mcp.options.v1.server).oauth`: the
/// RFC 9728 protected resource, its authorization servers and global scopes,
/// plus what the built-in JWKS verifier needs.
public struct OAuthResourceServer: Sendable {
    /// RFC 9728 well-known prefix for protected resource metadata.
    public static let metadataPrefix = "/.well-known/oauth-protected-resource"

    public let resourceURL: String
    public let authorizationServers: [String]
    public let scopes: [String]
    public let issuer: String
    public let audience: String
    public let jwksURL: String
    public let mcpPath: String
    /// Path-scoped metadata path, for example /.well-known/oauth-protected-resource/mcp.
    public let metadataPath: String
    /// Absolute URL of the path-scoped metadata, advertised in every challenge.
    public let metadataURL: String

    public init(
        resourceURL: String,
        authorizationServers: [String],
        scopes: [String] = [],
        issuer: String = "",
        audience: String = "",
        jwksURL: String = "",
        mcpPath: String = ""
    ) throws {
        guard let resource = URLComponents(string: resourceURL), resource.scheme == "https", let host = resource.host, !host.isEmpty else {
            throw RuntimeError("mcpruntime: invalid HTTPS resource URL \"\(resourceURL)\"")
        }
        if authorizationServers.isEmpty {
            throw RuntimeError("mcpruntime: at least one authorization server is required")
        }
        for server in authorizationServers {
            guard let parsed = URLComponents(string: server), parsed.scheme == "https", parsed.host?.isEmpty == false else {
                throw RuntimeError("mcpruntime: invalid HTTPS authorization server \"\(server)\"")
            }
        }
        let path = mcpPath.isEmpty ? "/mcp" : mcpPath
        if !path.hasPrefix("/") || path == "/" || path.hasPrefix("/.well-known/") {
            throw RuntimeError("mcpruntime: invalid MCP path \"\(path)\"")
        }
        var resourcePath = resource.percentEncodedPath
        while resourcePath.hasSuffix("/") {
            resourcePath.removeLast()
        }
        var authority = host
        if let port = resource.port {
            authority += ":\(port)"
        }
        self.resourceURL = resourceURL
        self.authorizationServers = authorizationServers
        self.scopes = scopes
        self.issuer = issuer
        self.audience = audience
        self.jwksURL = jwksURL
        self.mcpPath = path
        self.metadataPath = Self.metadataPrefix + resourcePath
        self.metadataURL = "https://\(authority)\(self.metadataPath)"
    }

    /// The built-in RS256 JWKS verifier for this policy.
    public func jwksVerifier() throws -> JWKSTokenVerifier {
        if jwksURL.isEmpty {
            throw RuntimeError("mcpruntime: OAuth needs a token verifier or a JWKS URL")
        }
        return try JWKSTokenVerifier(jwksURL: jwksURL, issuer: issuer, audience: audience)
    }

    /// RFC 9728 document whose scopes_supported adds every tool's required scopes.
    func metadataJSON(toolScopes: [String]) -> Data {
        var document: [String: Any] = [
            "resource": resourceURL,
            "authorization_servers": authorizationServers,
        ]
        let supported = Set(scopes).union(toolScopes).sorted()
        if !supported.isEmpty {
            document["scopes_supported"] = supported
        }
        return (try? JSONSerialization.data(withJSONObject: document, options: [.sortedKeys, .withoutEscapingSlashes])) ?? Data()
    }

    /// WWW-Authenticate value for a missing or invalid token.
    var bearerChallenge: String {
        "Bearer resource_metadata=\"\(metadataURL)\""
    }

    /// WWW-Authenticate value for 403 listing every scope the request needs.
    func insufficientScopeChallenge(_ required: Set<String>) -> String {
        "Bearer error=\"insufficient_scope\", scope=\"\(required.sorted().joined(separator: " "))\", resource_metadata=\"\(metadataURL)\""
    }
}

/// Scopes the tools/call messages in body need, including every message of a
/// batch, ported from mcpruntime/oauth_scope.go. Throws for a body that is not
/// exactly one JSON value, a tools/call without object params or string name,
/// or an ambiguous duplicate "name" key.
func requestedToolScopes(_ body: Data, toolScopes: [String: [String]]) throws -> Set<String> {
    let payload = try StrictJSON.parse(body)
    let messages: [StrictJSON.Node]
    if case .array(let batch) = payload {
        messages = batch
    } else {
        messages = [payload]
    }
    var scopes = Set<String>()
    for message in messages {
        guard case .string("tools/call")? = message["method"], message.has("id") else { continue }
        guard let params = message["params"], case .object = params else {
            throw StrictJSON.Failure(reason: "tools/call params must be an object")
        }
        guard case .string(let name)? = params["name"] else {
            throw StrictJSON.Failure(reason: "tools/call name must be a string")
        }
        scopes.formUnion(toolScopes[name] ?? [])
    }
    return scopes
}
