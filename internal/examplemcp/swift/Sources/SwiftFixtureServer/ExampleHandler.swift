import Foundation
import ProtocGenMCPRuntime
import SwiftProtobuf

/// Swift counterpart of the Go and JVM example handlers.
struct ExampleHandler: ExampleAPIToolHandler {
    private static let invalidOutput = ProcessInfo.processInfo.environment["PROTOC_GEN_MCP_INVALID_OUTPUT"] == "create_report"

    func createReport(_ context: ToolContext, _ request: Internal_Testproto_Example_V1_CreateReportRequest) async throws -> Internal_Testproto_Example_V1_CreateReportResponse {
        var response = Internal_Testproto_Example_V1_CreateReportResponse()
        response.reportID = "report-1"
        response.totalCount = 42
        response.status = .ok
        if Self.invalidOutput {
            return response
        }
        response.details = request.details
        response.warnings = ["none"]
        return response
    }

    func ping(_ context: ToolContext, _ request: Internal_Testproto_Example_V1_PingRequest) async throws -> Internal_Testproto_Example_V1_PingResponse {
        var response = Internal_Testproto_Example_V1_PingResponse()
        response.ack = SwiftProtobuf.Google_Protobuf_Empty()
        return response
    }

    func describeAdvancedShapes(_ context: ToolContext, _ request: Internal_Testproto_Example_V1_DescribeAdvancedShapesRequest) async throws -> Internal_Testproto_Example_V1_DescribeAdvancedShapesResponse {
        try echo(request)
    }

    func describeScalarShapes(_ context: ToolContext, _ request: Internal_Testproto_Example_V1_DescribeScalarShapesRequest) async throws -> Internal_Testproto_Example_V1_DescribeScalarShapesResponse {
        try echo(request)
    }

    func hiddenThing(_ context: ToolContext, _ request: Internal_Testproto_Example_V1_HiddenThingRequest) async throws -> Internal_Testproto_Example_V1_HiddenThingResponse {
        Internal_Testproto_Example_V1_HiddenThingResponse()
    }

    /// The shape tools return what they were given, field for field.
    private func echo<Response: SwiftProtobuf.Message>(_ request: some SwiftProtobuf.Message) throws -> Response {
        var options = JSONDecodingOptions()
        options.ignoreUnknownFields = true
        return try Response(jsonUTF8Data: try request.jsonUTF8Data(), options: options)
    }
}
