package com.example.todo.kotlin

import com.example.todo.v1.TodoAPIToolHandler
import com.example.todo.v1.TodoMCPHandlers
import com.example.todo.v1.TodoPromptHandler
import com.example.todo.v1.TodoProto
import com.example.todo.v1.TodoResourceHandler
import com.example.todo.v1.installTodoMCPHttp
import com.example.todo.v1.newTodoMCPServer
import io.ktor.server.cio.CIO
import io.ktor.server.engine.embeddedServer
import io.modelcontextprotocol.kotlin.sdk.server.ClientConnection
import io.modelcontextprotocol.kotlin.sdk.server.StdioServerTransport
import io.modelcontextprotocol.kotlin.sdk.types.PromptMessage
import io.modelcontextprotocol.kotlin.sdk.types.Resource
import io.modelcontextprotocol.kotlin.sdk.types.Role
import io.modelcontextprotocol.kotlin.sdk.types.TextContent
import java.time.Instant
import java.util.Collections
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.runBlocking
import kotlinx.io.asSink
import kotlinx.io.asSource
import kotlinx.io.buffered
import tech.easyp.mcp.runtime.oauth.AccessToken
import tech.easyp.mcp.runtime.oauth.TokenVerifier

// Local development tokens for --demo-auth. Never accept static tokens in
// production: without --demo-auth the server verifies RS256 JWTs against the
// JWKS declared in todo.proto.
private val demoScopes = mapOf(
    "demo-read" to listOf("todo:read"),
    "demo-write" to listOf("todo:read", "todo:write"),
)

fun main(args: Array<String>) = runBlocking {
    disableKotlinLoggingStartupMessage()
    var http = false
    var demoAuth = false
    var addr = "127.0.0.1:8080"
    val rest = args.iterator()
    while (rest.hasNext()) {
        when (val arg = rest.next()) {
            "--http" -> http = true
            "--demo-auth" -> demoAuth = true
            "--addr" -> addr = rest.next()
            else -> error("unknown argument: $arg")
        }
    }

    val handler = Handler()
    val handlers = TodoMCPHandlers(todoAPI = handler, prompts = Prompts(handler), resources = Resources(handler))
    if (http) {
        serveHttp(handlers, addr, demoAuth)
        return@runBlocking
    }

    val closed = CompletableDeferred<Unit>()
    val server = newTodoMCPServer(handlers)
    server.onClose { closed.complete(Unit) }

    val transport = StdioServerTransport(
        System.`in`.asSource().buffered(),
        System.out.asSink().buffered(),
    )
    server.createSession(transport)
    closed.await()
}

private suspend fun serveHttp(handlers: TodoMCPHandlers, addr: String, demoAuth: Boolean) {
    val host = addr.substringBeforeLast(':')
    val port = addr.substringAfterLast(':').toInt()
    require(!demoAuth || host in listOf("127.0.0.1", "localhost", "::1")) { "--demo-auth only listens on loopback addresses" }
    val verifier = if (demoAuth) {
        TokenVerifier { token -> demoScopes[token]?.let { AccessToken.of(token, it, Instant.now().plusSeconds(3600)) } }
    } else {
        null
    }
    val server = embeddedServer(CIO, host = host, port = port) {
        installTodoMCPHttp(handlers, verifier)
    }
    server.startSuspend(wait = false)
    println("listening on http://$host:${server.engine.resolvedConnectors().first().port}")
    System.out.flush()
    awaitCancellation()
}

private fun disableKotlinLoggingStartupMessage() {
    runCatching {
        val configClass = Class.forName("io.github.oshai.kotlinlogging.KotlinLoggingConfiguration")
        val instance = configClass.getField("INSTANCE").get(null)
        val setter = configClass.getMethod("setLogStartupMessage", Boolean::class.javaPrimitiveType)
        setter.invoke(instance, false)
    }
}

private class Handler : TodoAPIToolHandler {
    // HTTP requests run concurrently, so the list is synchronized.
    val tasks: MutableMap<String, TodoProto.Task> = Collections.synchronizedMap(linkedMapOf())
    private var nextId = 1

    fun snapshot(): List<TodoProto.Task> = synchronized(tasks) { tasks.values.toList() }

    override suspend fun createTask(
        ctx: ClientConnection,
        request: TodoProto.CreateTaskRequest,
    ): TodoProto.CreateTaskResponse {
        val id = synchronized(this) { "task-${nextId++}" }
        val task = TodoProto.Task.newBuilder()
            .setId(id)
            .setTitle(request.title)
            .addAllLabels(request.labelsList)
            .build()
        tasks[id] = task
        return TodoProto.CreateTaskResponse.newBuilder()
            .setTask(task)
            .build()
    }

    override suspend fun listTasks(
        ctx: ClientConnection,
        request: TodoProto.ListTasksRequest,
    ): TodoProto.ListTasksResponse {
        val includeDone = !request.hasIncludeDone() || request.includeDone
        val response = TodoProto.ListTasksResponse.newBuilder()
        snapshot()
            .filter { includeDone || !it.done }
            .forEach { response.addTasks(it) }
        return response.build()
    }

    override suspend fun completeTask(
        ctx: ClientConnection,
        request: TodoProto.CompleteTaskRequest,
    ): TodoProto.CompleteTaskResponse {
        val existing = tasks[request.id] ?: error("unknown task id: ${request.id}")
        val completed = existing.toBuilder().setDone(true).build()
        tasks[completed.id] = completed
        return TodoProto.CompleteTaskResponse.newBuilder()
            .setTask(completed)
            .build()
    }

    override suspend fun deleteTask(
        ctx: ClientConnection,
        request: TodoProto.DeleteTaskRequest,
    ): TodoProto.DeleteTaskResponse =
        TodoProto.DeleteTaskResponse.newBuilder()
            .setDeleted(tasks.remove(request.id) != null)
            .build()
}

private class Resources(private val handler: Handler) : TodoResourceHandler {
    override suspend fun listTasks(): List<Resource> =
        handler.snapshot().map { Resource(uri = "todo://tasks/${it.id}", name = "task", title = it.title) }

    override suspend fun readTask(id: String): TodoProto.Task =
        handler.tasks[id] ?: error("unknown task id: $id")
}

private class Prompts(private val handler: Handler) : TodoPromptHandler {
    override suspend fun planDay(arguments: Map<String, String>): List<PromptMessage> {
        val open = handler.snapshot().filterNot { it.done }.joinToString("") { "\n- ${it.title}" }
        return listOf(
            PromptMessage(
                role = Role.User,
                content = TextContent(text = "Plan the day around: ${arguments["focus"]}\n\nOpen tasks:$open"),
            ),
        )
    }
}
