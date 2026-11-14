import Foundation
import MCP
import XCTest

@testable import ProtocGenMCPHTTP
@testable import ProtocGenMCPRuntime

final class SchemaValidatorTests: XCTestCase {
    private func validator(_ json: String) throws -> SchemaValidator {
        try SchemaValidator(schemaJSON: json)
    }

    func testObjectKeywords() throws {
        let schema = try validator(#"{"type":"object","properties":{"city":{"type":"string","minLength":1},"count":{"type":"integer","minimum":1}},"required":["city"],"additionalProperties":false}"#)
        XCTAssertNil(schema.validate(.object(["city": .string("Paris"), "count": .int(2)])))
        XCTAssertNotNil(schema.validate(.object(["count": .int(2)])))
        XCTAssertNotNil(schema.validate(.object(["city": .string("")])))
        XCTAssertNotNil(schema.validate(.object(["city": .string("Paris"), "count": .int(0)])))
        XCTAssertNotNil(schema.validate(.object(["city": .string("Paris"), "extra": .bool(true)])))
        XCTAssertNotNil(schema.validate(.object(["city": .int(1)])))
        XCTAssertNil(schema.validate(.object(["city": .string("Paris"), "count": .double(2.0)])))
    }

    func testCompositionRefAndArrays() throws {
        let schema = try validator(##"{"$defs":{"Tag":{"type":"string","pattern":"^[a-z]+$"}},"type":"object","properties":{"tags":{"type":"array","items":{"$ref":"#/$defs/Tag"},"uniqueItems":true,"maxItems":2},"mode":{"enum":["a","b"]},"value":{"anyOf":[{"type":"string"},{"type":"null"}]},"pick":{"oneOf":[{"type":"integer"},{"type":"string"}]},"labels":{"type":"object","propertyNames":{"pattern":"^[0-9]+$"},"additionalProperties":{"type":"string"}}}}"##)
        XCTAssertNil(schema.validate(.object(["tags": .array([.string("a"), .string("b")]), "mode": .string("a"), "value": .null, "pick": .int(1), "labels": .object(["1": .string("one")])])))
        XCTAssertNotNil(schema.validate(.object(["tags": .array([.string("A")])])))
        XCTAssertNotNil(schema.validate(.object(["tags": .array([.string("a"), .string("a")])])))
        XCTAssertNotNil(schema.validate(.object(["tags": .array([.string("a"), .string("b"), .string("c")])])))
        XCTAssertNotNil(schema.validate(.object(["mode": .string("c")])))
        XCTAssertNotNil(schema.validate(.object(["value": .int(1)])))
        XCTAssertNotNil(schema.validate(.object(["pick": .bool(true)])))
        XCTAssertNotNil(schema.validate(.object(["labels": .object(["x": .string("one")])])))
        XCTAssertNotNil(schema.validate(.object(["labels": .object(["1": .int(1)])])))
    }
}

final class ResourceAndNamingTests: XCTestCase {
    func testURITemplatesMatchAndPercentDecodeLikeGo() throws {
        let users = try URITemplate("users://{user_id}/profile")
        XCTAssertEqual(try users.match("users://ada%20l/profile"), ["user_id": "ada l"])
        XCTAssertEqual(try users.match("users://a+b/profile"), ["user_id": "a+b"])
        XCTAssertNil(try users.match("users://ada/settings"))
        XCTAssertNil(try users.match("users://a/b/profile"))
        XCTAssertThrowsError(try users.match("users://bad%zz/profile"))
        XCTAssertThrowsError(try users.match("users:///profile"))
        let documents = try URITemplate("projects://{project_id}/documents/{document_id}")
        XCTAssertEqual(try documents.match("projects://p/documents/d"), ["project_id": "p", "document_id": "d"])
        XCTAssertNil(try URITemplate("docs://v1.0/{name}").match("docs://v1x0/a"))
    }

    func testToolAndPromptNamesJoinNamespacesWithoutDots() {
        XCTAssertEqual(toolName("raw.pb", "Create", defaultNamespace: "x"), "raw_pb_Create")
        XCTAssertEqual(toolName("", "Create", defaultNamespace: "x"), "Create")
        XCTAssertEqual(toolName(nil, "Create", defaultNamespace: "example.default"), "example_default_Create")
        XCTAssertEqual(promptName(nil, "code_review"), "code_review")
        XCTAssertEqual(promptName("team.a", "review"), "team_a_review")
    }

    func testProtoJSONCompletesDefaultsRecursively() {
        let shapes = ProtoJSONShapes([
            "pkg.Report": [
                ProtoJSONField("id", default: .string("")),
                ProtoJSONField("total", default: .string("0")),
                ProtoJSONField("tags", default: .array([])),
                ProtoJSONField("child", nested: "pkg.Child"),
                ProtoJSONField("children", default: .array([]), nested: "pkg.Child", .list),
                ProtoJSONField("byName", default: .object([:]), nested: "pkg.Child", .map),
            ],
            "pkg.Child": [ProtoJSONField("ok", default: .bool(false))],
        ])
        let completed = shapes.complete(
            .object([
                "id": .string("r1"),
                "child": .object([:]),
                "children": .array([.object([:])]),
                "byName": .object(["a": .object([:])]),
            ]),
            type: "pkg.Report"
        )
        XCTAssertEqual(
            completed,
            .object([
                "id": .string("r1"),
                "total": .string("0"),
                "tags": .array([]),
                "child": .object(["ok": .bool(false)]),
                "children": .array([.object(["ok": .bool(false)])]),
                "byName": .object(["a": .object(["ok": .bool(false)])]),
            ])
        )
    }

    func testAppsExtensionIsAddedToInitializeOnly() throws {
        let initialize = Data(#"{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"s","version":"1"}}}"#.utf8)
        guard case .object(let message) = try jsonValue(addAppsExtension(initialize)),
            case .object(let result)? = message["result"],
            case .object(let capabilities)? = result["capabilities"]
        else {
            return XCTFail("initialize result lost its shape")
        }
        XCTAssertEqual(capabilities["extensions"], .object([appExtensionID: .object(["mimeTypes": .array([.string(appHTMLMIMEType)])])]))
        XCTAssertEqual(capabilities["tools"], .object([:]))
        let other = Data(#"{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}"#.utf8)
        XCTAssertEqual(addAppsExtension(other), other)
    }

    func testToolGuardMatchesGo() {
        XCTAssertNil(toolGuardError(requiredScopes: [], token: nil))
        XCTAssertEqual(toolGuardError(requiredScopes: ["w"], token: nil), "permission denied: an authenticated token is required")
        let reader = AccessToken(token: "t", scopes: ["r"], expiresAt: nil)
        XCTAssertEqual(toolGuardError(requiredScopes: ["w"], token: reader), "permission denied: missing required scope \"w\"")
        XCTAssertNil(toolGuardError(requiredScopes: ["r"], token: reader))
    }

    func testGencodeVersionMismatchFails() {
        XCTAssertNoThrow(try checkGencodeVersion(GENCODE_VERSION))
        XCTAssertThrowsError(try checkGencodeVersion(GENCODE_VERSION + 1))
    }
}

final class ScopeGateTests: XCTestCase {
    private let tools = ["secure": ["reports:write"], "open": []]

    private func scopes(_ body: String) throws -> Set<String> {
        try requestedToolScopes(Data(body.utf8), toolScopes: tools)
    }

    func testCollectsScopesOfEveryCall() throws {
        XCTAssertEqual(try scopes(#"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"secure"}}"#), ["reports:write"])
        XCTAssertEqual(try scopes(#"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"open"}}"#), [])
        XCTAssertEqual(try scopes(#"{"jsonrpc":"2.0","method":"tools/call","params":{"name":"secure"}}"#), [])
        XCTAssertEqual(try scopes(#"[{"jsonrpc":"2.0","id":1,"method":"tools/list"},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"secure"}}]"#), ["reports:write"])
        XCTAssertEqual(try scopes(#"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"Name":"secure","name":"open"}}"#), [])
    }

    func testRejectsAmbiguousRequests() {
        for body in [
            #"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"open","name":"secure"}}"#,
            #"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"open"}} {}"#,
            #"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":[]}"#,
            #"{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":1}}"#,
            "{",
            "",
        ] {
            XCTAssertThrowsError(try scopes(body), body)
        }
    }

    func testResourceServerMetadataAndChallenges() throws {
        let oauth = try OAuthResourceServer(resourceURL: "https://mcp.example.com/mcp", authorizationServers: ["https://auth.example.com"], scopes: ["reports:read"])
        XCTAssertEqual(oauth.mcpPath, "/mcp")
        XCTAssertEqual(oauth.metadataPath, "/.well-known/oauth-protected-resource/mcp")
        XCTAssertEqual(oauth.metadataURL, "https://mcp.example.com/.well-known/oauth-protected-resource/mcp")
        XCTAssertEqual(
            String(decoding: oauth.metadataJSON(toolScopes: ["reports:write"]), as: UTF8.self),
            #"{"authorization_servers":["https://auth.example.com"],"resource":"https://mcp.example.com/mcp","scopes_supported":["reports:read","reports:write"]}"#
        )
        XCTAssertEqual(
            oauth.insufficientScopeChallenge(["reports:write", "reports:read"]),
            #"Bearer error="insufficient_scope", scope="reports:read reports:write", resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource/mcp""#
        )
        XCTAssertThrowsError(try OAuthResourceServer(resourceURL: "http://mcp.example.com", authorizationServers: ["https://a"]))
        XCTAssertThrowsError(try OAuthResourceServer(resourceURL: "https://mcp.example.com", authorizationServers: []))
        XCTAssertThrowsError(try OAuthResourceServer(resourceURL: "https://mcp.example.com", authorizationServers: ["https://a"], mcpPath: "/.well-known/x"))
        XCTAssertThrowsError(try oauth.jwksVerifier())
    }

    func testSameOriginMirrorsGo() {
        func request(_ headers: [String: String], method: String = "POST") -> HTTPRequest {
            HTTPRequest(method: method, headers: headers, body: nil, path: "/mcp")
        }
        XCTAssertTrue(MCPHTTPHandler.sameOrigin(request([:])))
        XCTAssertTrue(MCPHTTPHandler.sameOrigin(request(["Origin": "https://evil.example.com"], method: "GET")))
        XCTAssertTrue(MCPHTTPHandler.sameOrigin(request(["Origin": "http://127.0.0.1:8080", "Host": "127.0.0.1:8080"])))
        XCTAssertFalse(MCPHTTPHandler.sameOrigin(request(["Origin": "https://evil.example.com", "Host": "127.0.0.1:8080"])))
        XCTAssertFalse(MCPHTTPHandler.sameOrigin(request(["Sec-Fetch-Site": "cross-site"])))
    }
}
