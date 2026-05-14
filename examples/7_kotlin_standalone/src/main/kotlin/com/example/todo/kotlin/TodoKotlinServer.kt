package com.example.todo.kotlin

import com.example.todo.v1.TodoAPIToolHandler
import com.example.todo.v1.TodoProto
import com.example.todo.v1.registerTodoAPITools
import io.modelcontextprotocol.kotlin.sdk.server.ClientConnection
import io.modelcontextprotocol.kotlin.sdk.server.Server
import io.modelcontextprotocol.kotlin.sdk.server.ServerOptions
import io.modelcontextprotocol.kotlin.sdk.server.StdioServerTransport
import io.modelcontextprotocol.kotlin.sdk.types.Implementation
import io.modelcontextprotocol.kotlin.sdk.types.ServerCapabilities
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.runBlocking
import kotlinx.io.asSink
import kotlinx.io.asSource
import kotlinx.io.buffered

fun main() = runBlocking {
    disableKotlinLoggingStartupMessage()

    val closed = CompletableDeferred<Unit>()
    val server = Server(
        Implementation(name = "todo-kotlin-standalone", version = "0.1.0"),
        ServerOptions(ServerCapabilities(tools = ServerCapabilities.Tools(listChanged = false))),
    )
    server.onClose { closed.complete(Unit) }

    registerTodoAPITools(server, Handler(), namespace = "todo")

    val transport = StdioServerTransport(
        System.`in`.asSource().buffered(),
        System.out.asSink().buffered(),
    )
    server.createSession(transport)
    closed.await()
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
    private val tasks = linkedMapOf<String, TodoProto.Task>()
    private var nextId = 1

    override suspend fun createTask(
        ctx: ClientConnection,
        request: TodoProto.CreateTaskRequest,
    ): TodoProto.CreateTaskResponse {
        val id = "task-${nextId++}"
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
        tasks.values
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
}
