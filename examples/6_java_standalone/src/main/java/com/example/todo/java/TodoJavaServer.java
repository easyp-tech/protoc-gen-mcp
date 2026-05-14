package com.example.todo.java;

import com.example.todo.v1.TodoMcp;
import com.example.todo.v1.TodoProto;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.McpAsyncServerExchange;
import io.modelcontextprotocol.server.transport.StdioServerTransportProvider;
import java.util.LinkedHashMap;
import java.util.Map;

public final class TodoJavaServer {
  private TodoJavaServer() {
  }

  public static void main(String[] args) {
    StdioServerTransportProvider transportProvider =
        new StdioServerTransportProvider(McpJsonDefaults.getMapper());
    TodoMcp.registerTodoAPITools(transportProvider, new Handler(), "todo");
  }

  private static final class Handler implements TodoMcp.TodoAPIToolHandler {
    private final Map<String, TodoProto.Task> tasks = new LinkedHashMap<>();
    private int nextID = 1;

    @Override
    public TodoProto.CreateTaskResponse createTask(
        McpAsyncServerExchange ctx,
        TodoProto.CreateTaskRequest request
    ) {
      String id = "task-" + nextID++;
      TodoProto.Task task = TodoProto.Task.newBuilder()
          .setId(id)
          .setTitle(request.getTitle())
          .addAllLabels(request.getLabelsList())
          .build();
      tasks.put(id, task);
      return TodoProto.CreateTaskResponse.newBuilder()
          .setTask(task)
          .build();
    }

    @Override
    public TodoProto.ListTasksResponse listTasks(
        McpAsyncServerExchange ctx,
        TodoProto.ListTasksRequest request
    ) {
      boolean includeDone = !request.hasIncludeDone() || request.getIncludeDone();
      TodoProto.ListTasksResponse.Builder response = TodoProto.ListTasksResponse.newBuilder();
      for (TodoProto.Task task : tasks.values()) {
        if (includeDone || !task.getDone()) {
          response.addTasks(task);
        }
      }
      return response.build();
    }

    @Override
    public TodoProto.CompleteTaskResponse completeTask(
        McpAsyncServerExchange ctx,
        TodoProto.CompleteTaskRequest request
    ) {
      TodoProto.Task existing = tasks.get(request.getId());
      if (existing == null) {
        throw new IllegalArgumentException("unknown task id: " + request.getId());
      }
      TodoProto.Task completed = existing.toBuilder().setDone(true).build();
      tasks.put(completed.getId(), completed);
      return TodoProto.CompleteTaskResponse.newBuilder()
          .setTask(completed)
          .build();
    }
  }
}
