package internal.examplemcp.kotlin

import com.google.protobuf.ByteString
import internal.testproto.prompts.v1.PromptsPromptHandler
import internal.testproto.prompts.v1.registerPromptsPrompts
import internal.testproto.resources.v1.Resources
import internal.testproto.resources.v1.ResourcesResourceHandler
import internal.testproto.resources.v1.registerResourcesResources
import io.modelcontextprotocol.kotlin.sdk.server.Server
import io.modelcontextprotocol.kotlin.sdk.server.ServerOptions
import io.modelcontextprotocol.kotlin.sdk.server.StdioServerTransport
import io.modelcontextprotocol.kotlin.sdk.types.Implementation
import io.modelcontextprotocol.kotlin.sdk.types.PromptMessage
import io.modelcontextprotocol.kotlin.sdk.types.Resource
import io.modelcontextprotocol.kotlin.sdk.types.Role
import io.modelcontextprotocol.kotlin.sdk.types.ServerCapabilities
import io.modelcontextprotocol.kotlin.sdk.types.TextContent
import java.io.File
import kotlinx.coroutines.CompletableDeferred
import kotlinx.io.asSink
import kotlinx.io.asSource
import kotlinx.io.buffered

/** Kotlin counterpart of examplemcp.NewResourcesServer for the stdio resources contract. */
suspend fun runResourcesServer() {
    val closed = CompletableDeferred<Unit>()
    val server = Server(
        Implementation(name = "protoc-gen-mcp-kotlin-resources-server", version = "v0.0.1"),
        ServerOptions(
            ServerCapabilities(
                resources = ServerCapabilities.Resources(listChanged = false, subscribe = false),
                prompts = ServerCapabilities.Prompts(listChanged = false),
            ),
        ),
    )
    server.onClose { closed.complete(Unit) }

    registerResourcesResources(server, ResourcesFixtureHandler())
    registerPromptsPrompts(server, PromptsFixtureHandler())

    val transport = StdioServerTransport(
        System.`in`.asSource().buffered(),
        System.out.asSink().buffered(),
    )
    server.createSession(transport)
    closed.await()
}

private class ResourcesFixtureHandler : ResourcesResourceHandler {
    override suspend fun readServerStatus(): Resources.ServerStatus =
        Resources.ServerStatus.newBuilder().setHealthy(true).setUptimeSeconds(123).setVersion("v0.0.1").build()

    override suspend fun listUserProfiles(): List<Resource> =
        listOf(Resource(uri = "users://ada/profile", name = "user_profile", description = "User profile information"))

    override suspend fun readUserProfile(user_id: String): Resources.UserProfile =
        Resources.UserProfile.newBuilder()
            .setUserId(user_id)
            .setDisplayName("Ada Lovelace")
            .setEmail("$user_id@example.com")
            .setActive(true)
            .build()

    override suspend fun listDocuments(): List<Resource> = emptyList()

    override suspend fun readDocument(project_id: String, document_id: String): Resources.Document =
        Resources.Document.newBuilder()
            .setTitle("doc $document_id in $project_id")
            .setBody("body")
            .setAuthor("Ada")
            .build()

    override suspend fun readSkillDocument(): Resources.SkillDocument =
        Resources.SkillDocument.newBuilder()
            .setMarkdown(File("internal/examplemcp/testdata/SKILL.md").readText())
            .build()

    override suspend fun listPlainTexts(): List<Resource> =
        listOf(Resource(uri = "docs://example/intro", name = "plain_text", mimeType = "text/plain"))

    override suspend fun readPlainText(name: String): Resources.PlainText =
        Resources.PlainText.newBuilder().setBody("document: $name").build()

    override suspend fun readRawBinary(): Resources.RawBinary =
        Resources.RawBinary.newBuilder()
            .setData(ByteString.copyFrom(byteArrayOf(0, 1, 2, 255.toByte())))
            .build()

    override suspend fun readAppPage(): Any =
        File("internal/testproto/resources/v1/testdata/report.html").readText()
}

private fun userMessage(text: String): List<PromptMessage> =
    listOf(PromptMessage(role = Role.User, content = TextContent(text = text)))

private class PromptsFixtureHandler : PromptsPromptHandler {
    override suspend fun codeReview(arguments: Map<String, String>): List<PromptMessage> =
        userMessage("Review this ${arguments["language"]} code: ${arguments["code"]}")

    override suspend fun summarize(arguments: Map<String, String>): List<PromptMessage> =
        userMessage("Summarize: ${arguments["content"]}")

    override suspend fun explainError(arguments: Map<String, String>): List<PromptMessage> =
        userMessage("Explain: ${arguments["errorMessage"]}")
}
