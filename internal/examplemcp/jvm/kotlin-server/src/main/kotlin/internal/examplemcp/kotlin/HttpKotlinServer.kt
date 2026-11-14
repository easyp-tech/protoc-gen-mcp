package internal.examplemcp.kotlin

import internal.testproto.config.v1.SecureAPIToolHandler
import internal.testproto.config.v1.ServerConfig
import internal.testproto.config.v1.ServerConfigMCPHandlers
import internal.testproto.config.v1.installServerConfigMCPHttp
import io.ktor.server.cio.CIO
import io.ktor.server.engine.embeddedServer
import io.modelcontextprotocol.kotlin.sdk.server.ClientConnection
import java.time.Instant
import tech.easyp.mcp.runtime.kotlin.currentAccessToken
import tech.easyp.mcp.runtime.oauth.AccessToken

private val demoScopes = mapOf(
    "reader" to listOf("reports:read"),
    "writer" to listOf("reports:read", "reports:write"),
)

/** Kotlin counterpart of the Go HTTP fixture server for the shared HTTP contract. */
suspend fun runHttpServer() {
    val server = embeddedServer(CIO, host = "127.0.0.1", port = 0) {
        installServerConfigMCPHttp(ServerConfigMCPHandlers(secureAPI = SecureHandler())) { token ->
            demoScopes[token]?.let { AccessToken.of(token, it, Instant.now().plusSeconds(3600)) }
        }
    }
    server.startSuspend(wait = false)
    val port = server.engine.resolvedConnectors().first().port
    println("listening on http://127.0.0.1:$port")
    System.out.flush()
    kotlinx.coroutines.awaitCancellation()
}

private class SecureHandler : SecureAPIToolHandler {
    override suspend fun checkAccess(ctx: ClientConnection, request: ServerConfig.CheckAccessRequest): ServerConfig.CheckAccessResponse {
        check(currentAccessToken() != null) { "handler must see the bearer token" }
        return ServerConfig.CheckAccessResponse.newBuilder().setAllowed(true).build()
    }
}
