# JVM MCP Workspace

This workspace is the canonical runnable Java/Kotlin reference for JVM support
in this repository. It builds the local `protoc-gen-mcp` binary, generates
protobuf classes and JVM sidecars from `internal/testproto/example/v1/example.proto`,
and runs installable stdio servers against the official JVM MCP SDKs.

The workspace documents the tested paths for `lang=java` and `lang=kotlin`.
For release scope: this repository publishes the `protoc-gen-mcp binary`; it
does not publish separate Java or Kotlin runtime artifacts. Downstream JVM
projects compile generated sources against the official SDK coordinates used
here.

## Prerequisites

- Go 1.24+
- JDK 17+
- Gradle 9.2+

You do not need a separately installed `protoc` for this workspace. The
Gradle build uses the Maven artifact `com.google.protobuf:protoc:4.34.1`.

## Workspace Layout

- `build.gradle.kts` builds the local `protoc-gen-mcp` binary used by both JVM subprojects.
- `java-server/build.gradle.kts` generates protobuf Java output plus the `lang=java` MCP sidecar.
- `kotlin-server/build.gradle.kts` generates Java protobuf output, Kotlin protobuf output, and the `lang=kotlin` MCP sidecar.
- `java-server/src/main/java/.../ExampleJavaServer.java` and `kotlin-server/src/main/kotlin/.../ExampleKotlinServer.kt` are the executable stdio examples.

## Generate And Compile

Compile both JVM targets from the repository root:

```bash
gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin
```

The Java subproject passes `lang=java` to `protoc-gen-mcp` and compiles the
generated Java sidecar alongside protobuf Java output.

The Kotlin subproject passes `lang=kotlin` to `protoc-gen-mcp`. Its working
path is intentionally a dual protobuf generation flow:

- Java protobuf output
- Kotlin protobuf output
- `lang=kotlin` MCP sidecar

All three are required for the tested Kotlin build graph.

## Install Runnable Scripts

Build installable stdio scripts for both targets:

```bash
gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist
```

This produces the tested script entrypoints:

- `examples/jvm/java-server/build/install/java-server/bin/java-server`
- `examples/jvm/kotlin-server/build/install/kotlin-server/bin/kotlin-server`

Use these installed scripts as the canonical stdio launch path for manual
checks. They match the repository's automated runtime proof.

## Run

From the repository root, start the installed Java server:

```bash
examples/jvm/java-server/build/install/java-server/bin/java-server
```

Start the installed Kotlin server:

```bash
examples/jvm/kotlin-server/build/install/kotlin-server/bin/kotlin-server
```

The example servers register generated tools through the current public JVM
APIs:

- Java uses `ExampleMcp.registerExampleAPITools(transportProvider, new Handler(), "example")`
- Kotlin uses `registerExampleAPITools(server, Handler(), namespace = "example")`

## Verify

Run the same stdio parity proof used by the repository:

```bash
go test ./internal/examplemcp -run 'Test(Java|Kotlin).*OverStdio' -count=1
```

The tests validate:

- `tools/list` parity for the generated JVM bindings
- representative `tools/call` behavior
- invalid input rejection
- invalid output validation failure

For the full repository-level JVM truth path, run:

```bash
gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin
gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist
go test ./internal/examplemcp -run 'Test(Java|Kotlin).*OverStdio' -count=1
```
