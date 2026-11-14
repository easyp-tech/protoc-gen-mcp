import Foundation
import HTTPTypes
import Hummingbird
import MCP
import ProtocGenMCPHTTP

/// Serves handler with Hummingbird; any HTTP framework works the same way:
/// turn the request into MCP.HTTPRequest and copy MCPHTTPResponse back.
func serveHTTP(_ handler: MCPHTTPHandler, host: String, port: Int) async throws {
    let app = Application(
        responder: MCPResponder(handler: handler),
        configuration: .init(address: .hostname(host, port: port)),
        onServerRunning: { channel in
            print("listening on http://\(host):\(channel.localAddress?.port ?? port)")
            fflush(stdout)
        }
    )
    try await app.runService()
}

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
