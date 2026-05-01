plugins {
    application
    kotlin("jvm") version "2.3.10"
    java
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

kotlin {
    jvmToolchain(17)
    sourceSets.named("main") {
        kotlin.srcDir("src/generated/main/kotlin")
    }
}

sourceSets {
    named("main") {
        java {
            srcDir("src/generated/main/java")
        }
    }
}

application {
    mainClass.set("com.example.todo.kotlin.TodoKotlinServerKt")
}

tasks.register<Exec>("easypModDownload") {
    group = "build"
    description = "Download protobuf dependencies declared in easyp.yaml."
    commandLine("easyp", "--cfg", "easyp.yaml", "mod", "download")
}

tasks.register<Exec>("generateProto") {
    group = "build"
    description = "Generate protobuf Java/Kotlin classes and MCP Kotlin sidecars."
    dependsOn("easypModDownload")
    commandLine("easyp", "--cfg", "easyp.yaml", "generate", "-p", "proto", "-r", ".")
    doLast {
        delete("google", "src/generated/main/java/com/google")
    }
}

tasks.named("compileJava") {
    dependsOn("generateProto")
}

tasks.named("compileKotlin") {
    dependsOn("generateProto")
}

tasks.named("clean") {
    doLast {
        delete("src/generated", "google")
    }
}
