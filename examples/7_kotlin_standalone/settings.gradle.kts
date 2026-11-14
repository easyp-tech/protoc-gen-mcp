import org.gradle.api.initialization.resolve.RepositoriesMode

pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        google()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
        google()
    }
}

rootProject.name = "todo-kotlin-standalone"

// Use the runtime from this repository; projects outside it depend on the
// published tech.easyp:protoc-gen-mcp-runtime-kotlin instead.
includeBuild("../../runtime/jvm")
