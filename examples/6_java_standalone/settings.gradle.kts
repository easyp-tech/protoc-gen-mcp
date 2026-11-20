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

// Use the runtime from this repository. Projects outside it depend on
// tech.easyp:protoc-gen-mcp-runtime, which is not on Maven Central yet: build it with
// `gradle -p runtime/jvm publishToMavenLocal` and add mavenLocal().
includeBuild("../../runtime/jvm")
