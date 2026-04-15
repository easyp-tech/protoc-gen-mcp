import org.gradle.api.tasks.Exec

plugins {
    id("com.google.protobuf") version "0.9.6" apply false
    id("org.jetbrains.kotlin.jvm") version "2.3.10" apply false
}

val repoRoot = rootDir.resolve("../..").canonicalFile
val jvmBuildRoot = repoRoot.parentFile.resolve(".protoc-gen-mcp-jvm-build").canonicalFile
val protocGenMcpBinary = layout.buildDirectory.file("tools/protoc-gen-mcp")

layout.buildDirectory.set(jvmBuildRoot.resolve("root"))

tasks.register<Exec>("buildProtocGenMcp") {
    group = "build"
    description = "Build the local protoc-gen-mcp binary used by JVM compile gates."
    outputs.file(protocGenMcpBinary)
    doFirst {
        protocGenMcpBinary.get().asFile.parentFile.mkdirs()
    }
    workingDir = repoRoot
    commandLine("go", "build", "-o", protocGenMcpBinary.get().asFile.absolutePath, "./cmd/protoc-gen-mcp")
}
