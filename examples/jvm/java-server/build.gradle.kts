import com.google.protobuf.gradle.*
import org.gradle.api.tasks.Sync

plugins {
    application
    java
    id("com.google.protobuf")
}

dependencies {
    implementation("io.modelcontextprotocol.sdk:mcp:1.1.1")
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("com.google.protobuf:protobuf-java-util:4.34.1")
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

application {
    mainClass.set("internal.examplemcp.java.ExampleJavaServer")
}

val repoRoot = rootProject.projectDir.resolve("../..").canonicalFile
val protocGenMcpBinary = rootProject.layout.buildDirectory.file("tools/protoc-gen-mcp")
val stagedProtoDir = layout.buildDirectory.dir("staged-proto")
val stageProtoSources = tasks.register<Sync>("stageProtoSources") {
    from(repoRoot) {
        include("internal/testproto/example/v1/example.proto")
        include("mcp/options/v1/options.proto")
    }
    into(stagedProtoDir)
}

sourceSets {
    named("main") {
        proto {
            srcDir(stagedProtoDir)
        }
        java {
            srcDir(layout.buildDirectory.dir("generated/source/proto/main/java"))
            srcDir(layout.buildDirectory.dir("generated/source/proto/main/mcp"))
        }
    }
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:4.34.1"
    }
    plugins {
        id("mcp") {
            path = protocGenMcpBinary.get().asFile.absolutePath
        }
    }
    generateProtoTasks {
        ofSourceSet("main").configureEach {
            dependsOn(rootProject.tasks.named("buildProtocGenMcp"))
            dependsOn(stageProtoSources)
            plugins {
                id("mcp") {
                    option("paths=source_relative")
                    option("lang=java")
                }
            }
        }
    }
}

tasks.named("compileJava") {
    dependsOn(tasks.named("generateProto"))
}

tasks.named("processResources") {
    dependsOn(stageProtoSources)
}
