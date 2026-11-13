package com.example.todo.java;

import com.example.todo.v1.TodoMcp;
import com.example.todo.v1.TodoProto;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.McpAsyncServerExchange;
import io.modelcontextprotocol.server.transport.StdioServerTransportProvider;
import io.modelcontextprotocol.spec.McpSchema;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class TodoJavaServer {
  private TodoJavaServer() {
  }

  public static void main(String[] args) {
    StdioServerTransportProvider transportProvider =
        new StdioServerTransportProvider(McpJsonDefaults.getMapper());
    Handler handler = new Handler();
    TodoMcp.registerTodoAPITools(transportProvider, handler, "todo");
    TodoMcp.registerTodoResources(transportProvider, new Resources(handler), null);
    TodoMcp.registerTodoPrompts(transportProvider, new Prompts(handler), null);
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

  private static final class Resources implements TodoMcp.TodoResourceHandler {
    private final Handler handler;

    private Resources(Handler handler) {
      this.handler = handler;
    }

    @Override
    public List<McpSchema.Resource> listTasks() {
      List<McpSchema.Resource> resources = new ArrayList<>();
      for (TodoProto.Task task : handler.tasks.values()) {
        resources.add(McpSchema.Resource.builder()
            .name("task")
            .uri("todo://tasks/" + task.getId())
            .title(task.getTitle())
            .build());
      }
      return resources;
    }

    @Override
    public TodoProto.Task readTask(String id) {
      TodoProto.Task task = handler.tasks.get(id);
      if (task == null) {
        throw new IllegalArgumentException("unknown task id: " + id);
      }
      return task;
    }
  }

  private static final class Prompts implements TodoMcp.TodoPromptHandler {
    private final Handler handler;

    private Prompts(Handler handler) {
      this.handler = handler;
    }

    @Override
    public List<Map<String, Object>> planDay(Map<String, String> arguments) {
      StringBuilder text = new StringBuilder("Plan the day around: " + arguments.get("focus") + "\n\nOpen tasks:");
      for (TodoProto.Task task : handler.tasks.values()) {
        if (!task.getDone()) {
          text.append("\n- ").append(task.getTitle());
        }
      }
      return List.of(Map.of("role", "user", "content", Map.of("type", "text", "text", text.toString())));
    }
  }
}
