import Foundation
import HTTPTypes
import Hummingbird
import MCP
import ProtocGenMCPHTTP
import ProtocGenMCPRuntime

/// Swift counterpart of the Go HTTP fixture server for the shared HTTP contract.
func runHTTPServer() async throws {
    let demoScopes = ["reader": ["reports:read"], "writer": ["reports:read", "reports:write"]]
    let handler = try newServerConfigMCPHTTPHandler(
        ServerConfigMCPHandlers(secureAPI: SecureHandler()),
        verifier: DemoVerifier(scopes: demoScopes)
    )
    let app = Application(
        responder: MCPResponder(handler: handler),
        configuration: .init(address: .hostname("127.0.0.1", port: 0)),
        onServerRunning: { channel in
            print("listening on http://127.0.0.1:\(channel.localAddress?.port ?? 0)")
            fflush(stdout)
        }
    )
    try await app.runService()
}

struct DemoVerifier: TokenVerifier {
    let scopes: [String: [String]]

    func verify(_ token: String) async throws -> AccessToken? {
        guard let granted = scopes[token] else { return nil }
        return AccessToken(token: token, clientID: "contract", scopes: granted, expiresAt: Date().addingTimeInterval(3600))
    }
}

struct SecureHandler: SecureAPIToolHandler {
    func checkAccess(_ context: ToolContext, _ request: Internal_Testproto_Config_V1_CheckAccessRequest) async throws -> Internal_Testproto_Config_V1_CheckAccessResponse {
        guard context.accessToken != nil else {
            throw RuntimeError("handler must see the bearer token")
        }
        var response = Internal_Testproto_Config_V1_CheckAccessResponse()
        response.allowed = true
        return response
    }
}

/// Adapts Hummingbird requests to the framework-neutral MCPHTTPHandler.
struct MCPResponder: HTTPResponder {
    typealias Context = BasicRequestContext
    let handler: MCPHTTPHandler

    func respond(to request: Request, context: Context) async throws -> Response {
        var collected: ByteBuffer
        do {
            collected = try await request.body.collect(upTo: MCPHTTPHandler.maxRequestBodyBytes + 1)
        } catch {
            return Response(status: .contentTooLarge, body: .init(byteBuffer: ByteBuffer(string: "MCP request body too large\n")))
        }
        var headers: [String: String] = [:]
        for field in request.headers {
            let name = field.name.canonicalName
            headers[name] = headers[name].map { "\($0), \(field.value)" } ?? field.value
        }
        let body = collected.readData(length: collected.readableBytes) ?? Data()
        let response = await handler.handle(MCP.HTTPRequest(method: request.method.rawValue, headers: headers, body: body, path: request.uri.path))
        var fields = HTTPFields()
        for (name, value) in response.headers {
            if let fieldName = HTTPField.Name(name) {
                fields[fieldName] = value
            }
        }
        let status = HTTPTypes.HTTPResponse.Status(code: response.status)
        switch response.body {
        case .data(let data):
            return Response(status: status, headers: fields, body: .init(byteBuffer: ByteBuffer(data: data)))
        case .stream(let stream):
            return Response(status: status, headers: fields, body: .init(asyncSequence: stream.map { ByteBuffer(data: $0) }))
        }
    }
}
