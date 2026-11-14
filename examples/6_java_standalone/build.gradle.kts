plugins {
    application
    java
}

dependencies {
    implementation("tech.easyp:protoc-gen-mcp-runtime:0.8.0")
    implementation("io.modelcontextprotocol.sdk:mcp:1.1.1")
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("com.google.protobuf:protobuf-java-util:4.34.1")
    implementation("org.eclipse.jetty.ee10:jetty-ee10-servlet:12.1.12")
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

application {
    mainClass.set("com.example.todo.java.TodoJavaServer")
}

sourceSets {
    named("main") {
        java {
            srcDir("src/generated/main/java")
        }
    }
}

tasks.register<Exec>("easypModDownload") {
    group = "build"
    description = "Download protobuf dependencies declared in easyp.yaml."
    commandLine("easyp", "--cfg", "easyp.yaml", "mod", "download")
}

tasks.register<Exec>("generateProto") {
    group = "build"
    description = "Generate protobuf Java classes and MCP Java sidecars."
    dependsOn("easypModDownload")
    commandLine("easyp", "--cfg", "easyp.yaml", "generate", "-p", "proto", "-r", ".")
    doLast {
        delete("google", "src/generated/main/java/com/google")
    }
}

tasks.named("compileJava") {
    dependsOn("generateProto")
}

tasks.named("clean") {
    doLast {
        delete("src/generated", "google")
    }
}
