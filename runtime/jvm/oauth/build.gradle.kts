import com.vanniktech.maven.publish.JavadocJar
import com.vanniktech.maven.publish.JavaLibrary

plugins {
    `java-library`
    id("com.vanniktech.maven.publish")
}

dependencies {
    implementation("com.nimbusds:nimbus-jose-jwt:10.10")

    testImplementation(platform("org.junit:junit-bom:5.13.4"))
    testImplementation("org.junit.jupiter:junit-jupiter")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

tasks.withType<Javadoc> {
    (options as StandardJavadocDocletOptions).addStringOption("Xdoclint:none", "-quiet")
}

mavenPublishing {
    configure(JavaLibrary(javadocJar = JavadocJar.Javadoc(), sourcesJar = true))
    publishToMavenCentral()
    if (providers.gradleProperty("signingInMemoryKey").isPresent) {
        signAllPublications()
    }
    coordinates("tech.easyp", "protoc-gen-mcp-runtime-oauth", version.toString())
    pom {
        name.set("protoc-gen-mcp-runtime-oauth")
        description.set("OAuth resource server support shared by the JVM runtimes of protoc-gen-mcp.")
        url.set("https://github.com/easyp-tech/protoc-gen-mcp")
        licenses {
            license {
                name.set("Apache-2.0")
                url.set("https://www.apache.org/licenses/LICENSE-2.0")
            }
        }
        developers {
            developer {
                id.set("easyp-tech")
                name.set("easyp-tech")
                url.set("https://github.com/easyp-tech")
            }
        }
        scm {
            url.set("https://github.com/easyp-tech/protoc-gen-mcp")
            connection.set("scm:git:https://github.com/easyp-tech/protoc-gen-mcp.git")
            developerConnection.set("scm:git:ssh://git@github.com/easyp-tech/protoc-gen-mcp.git")
        }
    }
}
