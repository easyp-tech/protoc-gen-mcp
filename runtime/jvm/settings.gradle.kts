import org.gradle.api.initialization.resolve.RepositoriesMode

pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
    }
}

rootProject.name = "protoc-gen-mcp-runtime-jvm"

include("protoc-gen-mcp-runtime", "protoc-gen-mcp-runtime-kotlin")
project(":protoc-gen-mcp-runtime").projectDir = file("runtime")
project(":protoc-gen-mcp-runtime-kotlin").projectDir = file("runtime-kotlin")
