package com.example.todo.java;

import com.example.todo.v1.TodoMcp;
import com.example.todo.v1.TodoProto;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.McpAsyncServerExchange;
import io.modelcontextprotocol.server.transport.StdioServerTransportProvider;
import io.modelcontextprotocol.spec.McpSchema;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.eclipse.jetty.ee10.servlet.ServletContextHandler;
import org.eclipse.jetty.ee10.servlet.ServletHolder;
import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.server.ServerConnector;
import tech.easyp.mcp.runtime.McpHttpServer;
import tech.easyp.mcp.runtime.oauth.AccessToken;
import tech.easyp.mcp.runtime.oauth.TokenVerifier;

public final class TodoJavaServer {
  private TodoJavaServer() {
  }

  // Local development tokens for --demo-auth. Never accept static tokens in
  // production: without --demo-auth the server verifies RS256 JWTs against the
  // JWKS declared in todo.proto.
  private static final Map<String, List<String>> DEMO_SCOPES = Map.of(
      "demo-read", List.of("todo:read"),
      "demo-write", List.of("todo:read", "todo:write"));

  public static void main(String[] args) throws Exception {
    boolean http = false;
    boolean demoAuth = false;
    String addr = "127.0.0.1:8080";
    for (int index = 0; index < args.length; index++) {
      switch (args[index]) {
        case "--http" -> http = true;
        case "--demo-auth" -> demoAuth = true;
        case "--addr" -> addr = args[++index];
        default -> throw new IllegalArgumentException("unknown argument: " + args[index]);
      }
    }
    Handler handler = new Handler();
    TodoMcp.TodoMCPHandlers handlers = new TodoMcp.TodoMCPHandlers(handler, new Prompts(handler), new Resources(handler));
    if (!http) {
      TodoMcp.installTodoMCPServer(new StdioServerTransportProvider(McpJsonDefaults.getMapper()), handlers);
      return;
    }
    serveHttp(handlers, addr, demoAuth);
  }

  private static void serveHttp(TodoMcp.TodoMCPHandlers handlers, String addr, boolean demoAuth) throws Exception {
    String host = addr.substring(0, addr.lastIndexOf(':'));
    int port = Integer.parseInt(addr.substring(addr.lastIndexOf(':') + 1));
    if (demoAuth && !List.of("127.0.0.1", "localhost", "::1").contains(host)) {
      throw new IllegalArgumentException("--demo-auth only listens on loopback addresses");
    }
    TokenVerifier verifier = demoAuth
        ? token -> DEMO_SCOPES.containsKey(token) ? AccessToken.of(token, DEMO_SCOPES.get(token), Instant.now().plusSeconds(3600)) : null
        : null;
    McpHttpServer server = new McpHttpServer();
    TodoMcp.installTodoMCPServer(server, handlers);

    Server jetty = new Server();
    ServerConnector connector = new ServerConnector(jetty);
    connector.setHost(host);
    connector.setPort(port);
    jetty.addConnector(connector);
    ServletContextHandler context = new ServletContextHandler();
    context.addServlet(new ServletHolder(TodoMcp.newTodoMCPHttpServlet(server, verifier)), "/*");
    jetty.setHandler(context);
    jetty.start();
    System.out.println("listening on http://" + host + ":" + connector.getLocalPort());
    System.out.flush();
    jetty.join();
  }

  private static final class Handler implements TodoMcp.TodoAPIToolHandler {
    // HTTP requests run concurrently, so the list is synchronized.
    private final Map<String, TodoProto.Task> tasks = Collections.synchronizedMap(new LinkedHashMap<>());
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
      for (TodoProto.Task task : List.copyOf(tasks.values())) {
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

    @Override
    public TodoProto.DeleteTaskResponse deleteTask(
        McpAsyncServerExchange ctx,
        TodoProto.DeleteTaskRequest request
    ) {
      return TodoProto.DeleteTaskResponse.newBuilder()
          .setDeleted(tasks.remove(request.getId()) != null)
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
      for (TodoProto.Task task : List.copyOf(handler.tasks.values())) {
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
      for (TodoProto.Task task : List.copyOf(handler.tasks.values())) {
        if (!task.getDone()) {
          text.append("\n- ").append(task.getTitle());
        }
      }
      return List.of(Map.of("role", "user", "content", Map.of("type", "text", "text", text.toString())));
    }
  }
}
