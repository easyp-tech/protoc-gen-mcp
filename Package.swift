// swift-tools-version:6.1
// SwiftPM resolves this repository by git tag, so the Swift runtime for code
// generated with lang=swift is released together with protoc-gen-mcp.
import PackageDescription

let package = Package(
    name: "protoc-gen-mcp",
    platforms: [
        .macOS(.v13),
        .iOS(.v16),
    ],
    products: [
        .library(name: "ProtocGenMCPRuntime", targets: ["ProtocGenMCPRuntime"]),
        .library(name: "ProtocGenMCPHTTP", targets: ["ProtocGenMCPHTTP"]),
    ],
    dependencies: [
        .package(url: "https://github.com/modelcontextprotocol/swift-sdk.git", .upToNextMinor(from: "0.12.1")),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.39.0"),
        .package(url: "https://github.com/apple/swift-log.git", from: "1.5.0"),
        .package(url: "https://github.com/vapor/jwt-kit.git", from: "5.7.1"),
        .package(url: "https://github.com/apple/swift-crypto.git", "3.0.0"..<"5.0.0"),
    ],
    targets: [
        .target(
            name: "ProtocGenMCPRuntime",
            dependencies: [
                .product(name: "MCP", package: "swift-sdk"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
                .product(name: "Logging", package: "swift-log"),
            ],
            path: "runtime/swift/Sources/ProtocGenMCPRuntime"
        ),
        .target(
            name: "ProtocGenMCPHTTP",
            dependencies: [
                "ProtocGenMCPRuntime",
                .product(name: "MCP", package: "swift-sdk"),
                .product(name: "JWTKit", package: "jwt-kit"),
            ],
            path: "runtime/swift/Sources/ProtocGenMCPHTTP"
        ),
        .testTarget(
            name: "ProtocGenMCPRuntimeTests",
            dependencies: [
                "ProtocGenMCPRuntime",
                "ProtocGenMCPHTTP",
                .product(name: "MCP", package: "swift-sdk"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
                .product(name: "JWTKit", package: "jwt-kit"),
                .product(name: "_CryptoExtras", package: "swift-crypto"),
            ],
            path: "runtime/swift/Tests/ProtocGenMCPRuntimeTests"
        ),
    ]
)
