package tech.easyp.mcp.runtime.kotlin

import com.google.protobuf.Struct
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.contentType
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

class McpHttpTest {
    private fun scopedServer() = newConfiguredServer("kotlin-http-test", "v1.0.0", apps = false).also { server ->
        installMcpHandlers(server).addTool(
            RegisteredTool(
                name = "secure",
                title = "secure",
                description = null,
                inputSchemaJson = """{"type":"object"}""",
                outputSchemaJson = """{"type":"object"}""",
                requestBuilder = { Struct.newBuilder() },
                handler = { _, request -> request },
                annotations = null,
                icons = emptyList(),
                execution = null,
                requiredScopes = listOf("reports:write"),
            ),
        )
    }

    @Test
    fun openEndpointDeniesScopedToolsWithoutAToken() = testApplication {
        application { installMcpHttp(null, null) { scopedServer() } }
        val response = client.post("/mcp") {
            contentType(ContentType.Application.Json)
            header(HttpHeaders.Accept, "application/json, text/event-stream")
            setBody("""{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"secure","arguments":{}}}""")
        }
        val body = response.bodyAsText()
        assertEquals(HttpStatusCode.OK, response.status, body)
        val result = Json.parseToJsonElement(body).jsonObject["result"]!!.jsonObject
        assertTrue(result["isError"]!!.jsonPrimitive.boolean)
        assertEquals(
            "permission denied: an authenticated token is required",
            result["content"]!!.jsonArray[0].jsonObject["text"]!!.jsonPrimitive.content,
        )
        assertEquals(HttpStatusCode.NotFound, client.post("/other").status)
    }

    @Test
    fun crossOriginPostsAreRejected() = testApplication {
        application { installMcpHttp(null, null) { scopedServer() } }
        val response = client.post("/mcp") {
            contentType(ContentType.Application.Json)
            header(HttpHeaders.Origin, "https://evil.example.com")
            setBody("{}")
        }
        assertEquals(HttpStatusCode.Forbidden, response.status)
    }
}
