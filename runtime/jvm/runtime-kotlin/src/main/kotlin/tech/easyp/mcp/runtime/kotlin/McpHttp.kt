package tech.easyp.mcp.runtime.kotlin

import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpMethod
import io.ktor.http.HttpStatusCode
import io.ktor.http.withCharset
import io.ktor.serialization.kotlinx.json.json
import io.ktor.server.application.Application
import io.ktor.server.application.ApplicationCall
import io.ktor.server.application.install
import io.ktor.server.application.pluginOrNull
import io.ktor.server.plugins.contentnegotiation.ContentNegotiation
import io.ktor.server.plugins.doublereceive.DoubleReceive
import io.ktor.server.request.httpMethod
import io.ktor.server.request.path
import io.ktor.server.request.receiveChannel
import io.ktor.server.response.header
import io.ktor.server.response.respondBytes
import io.ktor.server.response.respondText
import io.ktor.server.routing.route
import io.ktor.server.routing.routing
import io.ktor.utils.io.readRemaining
import io.modelcontextprotocol.kotlin.sdk.server.Server
import io.modelcontextprotocol.kotlin.sdk.server.ServerSession
import io.modelcontextprotocol.kotlin.sdk.server.StreamableHttpServerTransport
import io.modelcontextprotocol.kotlin.sdk.types.McpJson
import java.net.URI
import java.time.Instant
import java.util.Collections
import java.util.TreeSet
import java.util.WeakHashMap
import kotlin.coroutines.AbstractCoroutineContextElement
import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.coroutineContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.io.readByteArray
import tech.easyp.mcp.runtime.oauth.AccessToken
import tech.easyp.mcp.runtime.oauth.OAuthResourceServer
import tech.easyp.mcp.runtime.oauth.ScopeGate
import tech.easyp.mcp.runtime.oauth.TokenVerifier

/** Carries the validated bearer token into the coroutine that runs a handler. */
class AccessTokenElement(val token: AccessToken) : AbstractCoroutineContextElement(Key) {
    companion object Key : CoroutineContext.Key<AccessTokenElement>
}

/** The bearer token of the HTTP request being served, or null on stdio and on an open endpoint. */
suspend fun currentAccessToken(): AccessToken? = coroutineContext[AccessTokenElement]?.token

private val sessionAccessTokens = Collections.synchronizedMap(WeakHashMap<ServerSession, AccessToken>())

internal fun accessTokenOf(session: ServerSession): AccessToken? = sessionAccessTokens[session]

/**
 * Serves the servers built by [factory] over stateless Streamable HTTP, ported
 * from mcpruntime.NewConfiguredHTTPHandler and NewOAuthResourceHandler. Each
 * request gets a fresh server and an SDK [StreamableHttpServerTransport], as
 * the SDK's own stateless endpoint does. With [oauth], this adds RFC 9728
 * metadata with live scopes_supported, bearer authentication and a per-tool
 * scope check that answers 403 insufficient_scope before any tool runs;
 * [verifier] defaults to the built-in RS256 JWKS verifier.
 */
fun Application.installMcpHttp(oauth: OAuthResourceServer?, verifier: TokenVerifier?, factory: suspend () -> Server) {
    val tokenVerifier = if (oauth == null) null else verifier ?: oauth.jwksVerifier()
    val mcpPath = oauth?.mcpPath() ?: "/mcp"
    if (pluginOrNull(ContentNegotiation) == null) {
        install(ContentNegotiation) { json(McpJson) }
    }
    val probeLock = Mutex()
    var probe: ServerToolRegistry? = null
    suspend fun registry(): ServerToolRegistry = probeLock.withLock {
        probe ?: (registryOf(factory()) ?: ServerToolRegistry()).also { probe = it }
    }

    routing {
        if (oauth != null) {
            route(OAuthResourceServer.METADATA_PATH) {
                handle { metadata(call, oauth, registry()) }
            }
            route(OAuthResourceServer.METADATA_PATH + "/{...}") {
                handle { metadata(call, oauth, registry()) }
            }
        }
        route(mcpPath) {
            install(DoubleReceive)
            handle {
                serveMcp(call, oauth, tokenVerifier, factory) { registry() }
            }
        }
    }
}

private suspend fun serveMcp(
    call: ApplicationCall,
    oauth: OAuthResourceServer?,
    verifier: TokenVerifier?,
    factory: suspend () -> Server,
    registry: suspend () -> ServerToolRegistry,
) {
    if (!sameOrigin(call)) {
        text(call, HttpStatusCode.Forbidden, "cross-origin request detected\n")
        return
    }
    var token: AccessToken? = null
    if (oauth != null) {
        token = authenticate(call, verifier!!)
        if (token == null) {
            call.response.header(HttpHeaders.WWWAuthenticate, oauth.bearerChallenge())
            text(call, HttpStatusCode.Unauthorized, "invalid token\n")
            return
        }
    }
    if (call.request.httpMethod != HttpMethod.Post) {
        if (oauth != null && ScopeGate.missing(oauth.scopes(), token!!.scopes()).isNotEmpty()) {
            insufficientScope(call, oauth, oauth.scopes())
            return
        }
        call.response.header(HttpHeaders.Allow, "POST")
        text(call, HttpStatusCode.MethodNotAllowed, "Method not allowed.\n")
        return
    }
    val limit = ScopeGate.MAX_REQUEST_BODY_BYTES
    val declared = call.request.headers[HttpHeaders.ContentLength]?.toLongOrNull()
    if (declared != null && declared > limit) {
        text(call, HttpStatusCode.PayloadTooLarge, "MCP request body too large\n")
        return
    }
    // DoubleReceive keeps what is read here for the SDK transport.
    val body = call.receiveChannel().readRemaining(limit.toLong() + 1).readByteArray()
    if (body.size > limit) {
        text(call, HttpStatusCode.PayloadTooLarge, "MCP request body too large\n")
        return
    }
    if (oauth != null) {
        val tools = registry()
        val required = TreeSet(oauth.scopes())
        try {
            required.addAll(ScopeGate.requestedToolScopes(body.decodeToString()) { name -> tools.tool(name)?.requiredScopes ?: emptyList() })
        } catch (error: IllegalArgumentException) {
            text(call, HttpStatusCode.BadRequest, "invalid MCP request\n")
            return
        }
        if (ScopeGate.missing(required, token!!.scopes()).isNotEmpty()) {
            insufficientScope(call, oauth, required)
            return
        }
    }
    val server = factory()
    val transport = StreamableHttpServerTransport(
        StreamableHttpServerTransport.Configuration(enableJsonResponse = true, maxRequestBodySize = limit.toLong()),
    ).also { it.setSessionIdGenerator(null) }
    val session = server.createSession(transport)
    if (token != null) {
        sessionAccessTokens[session] = token
    }
    transport.handleRequest(null, call)
}

private suspend fun insufficientScope(call: ApplicationCall, oauth: OAuthResourceServer, required: Collection<String>) {
    call.response.header(HttpHeaders.WWWAuthenticate, oauth.insufficientScopeChallenge(required))
    call.response.header(HttpHeaders.CacheControl, "no-store")
    text(call, HttpStatusCode.Forbidden, "insufficient scope\n")
}

private suspend fun authenticate(call: ApplicationCall, verifier: TokenVerifier): AccessToken? {
    val header = call.request.headers[HttpHeaders.Authorization] ?: return null
    if (header.length < 7 || !header.regionMatches(0, "Bearer ", 0, 7, ignoreCase = true)) {
        return null
    }
    val raw = header.substring(7).trim()
    if (raw.isEmpty()) {
        return null
    }
    val token = try {
        withContext(Dispatchers.IO) { verifier.verify(raw) }
    } catch (error: Exception) {
        null
    } ?: return null
    val expiresAt = token.expiresAt() ?: return null
    return if (expiresAt.isAfter(Instant.now())) token else null
}

private suspend fun metadata(call: ApplicationCall, oauth: OAuthResourceServer, registry: ServerToolRegistry) {
    if (!oauth.servesMetadata(call.request.path())) {
        text(call, HttpStatusCode.NotFound, "404 page not found\n")
        return
    }
    call.response.header(HttpHeaders.AccessControlAllowOrigin, "*")
    call.response.header(HttpHeaders.AccessControlAllowMethods, "GET, OPTIONS")
    call.response.header(HttpHeaders.AccessControlAllowHeaders, "Content-Type")
    when (call.request.httpMethod) {
        HttpMethod.Options -> call.respondBytes(ByteArray(0), status = HttpStatusCode.NoContent)
        HttpMethod.Get -> {
            val scopes = registry.allTools().flatMap { it.requiredScopes }
            call.respondText(oauth.metadataJson(scopes), ContentType.Application.Json, HttpStatusCode.OK)
        }
        else -> text(call, HttpStatusCode.MethodNotAllowed, "Method not allowed\n")
    }
}

/** Mirrors net/http CrossOriginProtection for state-changing requests. */
private fun sameOrigin(call: ApplicationCall): Boolean {
    if (call.request.httpMethod in listOf(HttpMethod.Get, HttpMethod.Head, HttpMethod.Options)) {
        return true
    }
    val site = call.request.headers["Sec-Fetch-Site"]
    if (site != null) {
        return site == "same-origin" || site == "none"
    }
    val origin = call.request.headers[HttpHeaders.Origin] ?: return true
    return try {
        URI.create(origin).rawAuthority == call.request.headers[HttpHeaders.Host]
    } catch (error: IllegalArgumentException) {
        false
    }
}

private suspend fun text(call: ApplicationCall, status: HttpStatusCode, message: String) {
    call.response.header("X-Content-Type-Options", "nosniff")
    call.respondText(message, ContentType.Text.Plain.withCharset(Charsets.UTF_8), status)
}
