import com.google.protobuf.gradle.*
import org.gradle.api.tasks.Sync

plugins {
    application
    id("org.jetbrains.kotlin.jvm")
    java
    id("com.google.protobuf")
}

dependencies {
    implementation("io.modelcontextprotocol:kotlin-sdk-server:0.11.1")
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("com.google.protobuf:protobuf-kotlin:4.34.1")
    implementation("com.google.protobuf:protobuf-java-util:4.34.1")
    implementation("com.networknt:json-schema-validator:1.5.9")
    runtimeOnly("org.slf4j:slf4j-nop:2.0.17")
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

application {
    mainClass.set("internal.examplemcp.kotlin.ExampleKotlinServerKt")
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
        }
    }
}

kotlin {
    jvmToolchain(17)
    sourceSets.named("main") {
        kotlin.srcDir(layout.buildDirectory.dir("generated/source/proto/main/kotlin"))
        kotlin.srcDir(layout.buildDirectory.dir("generated/source/proto/main/mcp"))
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
            builtins {
                id("kotlin") {}
            }
            plugins {
                id("mcp") {
                    option("paths=source_relative")
                    option("lang=kotlin")
                }
            }
        }
    }
}

tasks.named("compileKotlin") {
    dependsOn(tasks.named("generateProto"))
}

tasks.named("processResources") {
    dependsOn(stageProtoSources)
}
