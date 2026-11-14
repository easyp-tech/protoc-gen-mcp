import Foundation

/// A validated bearer token: the scopes it grants and when it expires.
public struct AccessToken: Sendable, Hashable {
    public var token: String
    public var clientID: String
    public var subject: String
    public var scopes: [String]
    public var expiresAt: Date?

    public init(token: String, clientID: String = "", subject: String = "", scopes: [String], expiresAt: Date?) {
        self.token = token
        self.clientID = clientID
        self.subject = subject
        self.scopes = scopes
        self.expiresAt = expiresAt
    }
}

/// What generated tool handlers receive with each call.
public struct ToolContext: Sendable {
    /// The bearer token of the HTTP request, or nil on stdio and on an open endpoint.
    public let accessToken: AccessToken?

    public init(accessToken: AccessToken?) {
        self.accessToken = accessToken
    }
}

/// Text of the in-call scope guard, or nil when the call may run: a scoped
/// tool needs a token (stdio has none) holding every required scope.
public func toolGuardError(requiredScopes: [String], token: AccessToken?) -> String? {
    if requiredScopes.isEmpty {
        return nil
    }
    guard let token else {
        return "permission denied: an authenticated token is required"
    }
    for scope in requiredScopes where !token.scopes.contains(scope) {
        return "permission denied: missing required scope \"\(scope)\""
    }
    return nil
}
