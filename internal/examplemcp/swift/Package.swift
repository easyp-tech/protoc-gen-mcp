// swift-tools-version:6.1
// Fixture servers for the shared stdio and HTTP contracts, built from the
// lang=swift sidecars of internal/testproto and the in-repo runtime.
import PackageDescription

let package = Package(
    name: "examplemcp-swift",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(name: "protoc-gen-mcp", path: "../../.."),
        .package(url: "https://github.com/modelcontextprotocol/swift-sdk.git", .upToNextMinor(from: "0.12.1")),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.39.0"),
        .package(url: "https://github.com/hummingbird-project/hummingbird.git", from: "2.28.0"),
        .package(url: "https://github.com/apple/swift-http-types.git", from: "1.0.0"),
    ],
    targets: [
        .executableTarget(
            name: "SwiftFixtureServer",
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
