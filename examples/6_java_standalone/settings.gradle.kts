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

rootProject.name = "todo-java-standalone"

// Use the runtime from this repository; projects outside it depend on the
// published tech.easyp:protoc-gen-mcp-runtime instead.
includeBuild("../../runtime/jvm")
