// swift-tools-version:6.1
import PackageDescription

let package = Package(
    name: "notebook-swift-standalone",
    platforms: [.macOS(.v14)],
    dependencies: [
        // Use the runtime from this repository; projects outside it depend on
        // .package(url: "https://github.com/easyp-tech/protoc-gen-mcp.git", from: "1.1.0").
        .package(name: "protoc-gen-mcp", path: "../.."),
        .package(url: "https://github.com/modelcontextprotocol/swift-sdk.git", .upToNextMinor(from: "0.12.1")),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.39.0"),
        .package(url: "https://github.com/hummingbird-project/hummingbird.git", from: "2.28.0"),
        .package(url: "https://github.com/apple/swift-http-types.git", from: "1.0.0"),
    ],
    targets: [
        .executableTarget(
            name: "NotebookServer",
            dependencies: [
                .product(name: "ProtocGenMCPRuntime", package: "protoc-gen-mcp"),
                .product(name: "ProtocGenMCPHTTP", package: "protoc-gen-mcp"),
                .product(name: "MCP", package: "swift-sdk"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
                .product(name: "Hummingbird", package: "hummingbird"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
    ]
)
