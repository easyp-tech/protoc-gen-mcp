package examples_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPythonExamplesOverStdio(t *testing.T) {
	root := repoRoot(t)
	examples := []struct {
		name string
		path string
		run  func(t *testing.T, session *mcp.ClientSession)
	}{
		{
			name: "helloworld",
			path: filepath.Join(root, "examples/1_helloworld/main.py"),
			run:  runHelloWorldExample,
		},
		{
			name: "weather",
			path: filepath.Join(root, "examples/2_weather_api/main.py"),
			run:  runWeatherExample,
		},
		{
			name: "file-manager",
			path: filepath.Join(root, "examples/3_file_manager/main.py"),
			run:  runFileManagerExample,
		},
		{
			name: "crm",
			path: filepath.Join(root, "examples/4_crm_system/main.py"),
			run:  runCRMExample,
		},
	}

	for _, tc := range examples {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := mcp.NewClient(&mcp.Implementation{
				Name:    "protoc-gen-mcp-python-examples-test-client",
				Version: "v0.0.1",
			}, nil)

			session, err := client.Connect(ctx, &mcp.CommandTransport{
				Command: pythonExampleCommand(t, root, tc.path),
			}, nil)
			if err != nil {
				t.Fatalf("client.Connect() over stdio failed: %v", err)
			}
			defer session.Close()

			tc.run(t, session)
		})
	}
}

func TestStandalonePythonExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/5_python_standalone")
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-standalone-python-example-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: standalonePythonExampleCommand(t, projectDir),
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	runStandaloneNotebookExample(t, session)
}

func TestStandalonePythonProtobufExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/10_python_protobuf_standalone")
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-standalone-python-protobuf-example-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: standalonePythonExampleCommand(t, projectDir),
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	runStandaloneTaskProtobufExample(t, session)
}

func runHelloWorldExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "greeter_SayHello")

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "greeter_SayHello",
		Arguments: map[string]any{
			"name": "Alice",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(SayHello) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("SayHello returned tool error: %+v", result)
	}
	assertTextStructuredContentMatch(t, "greeter_SayHello", result)
	structured := decodeMap(t, result.StructuredContent)
	if got := structured["message"]; got != "Hello, Alice!" {
		t.Fatalf("message = %v, want Hello, Alice!", got)
	}
}

func runWeatherExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "weather_GetCurrentWeather")

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "weather_GetCurrentWeather",
		Arguments: map[string]any{
			"city": "London",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(GetCurrentWeather) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetCurrentWeather returned tool error: %+v", result)
	}
	assertTextStructuredContentMatch(t, "weather_GetCurrentWeather", result)
	structured := decodeMap(t, result.StructuredContent)
	if got := structured["condition"]; got != "Cloudy" {
		t.Fatalf("condition = %v, want Cloudy", got)
	}
	if got := structured["temperature"]; got != 14.0 {
		t.Fatalf("temperature = %v, want 14", got)
	}
}

func runFileManagerExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "files_ReadFile")
	findTool(t, tools.Tools, "files_DeleteFile")

	readResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "files_ReadFile",
		Arguments: map[string]any{
			"filename": "example.txt",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(ReadFile) failed: %v", err)
	}
	if readResult.IsError {
		t.Fatalf("ReadFile returned tool error: %+v", readResult)
	}
	assertTextStructuredContentMatch(t, "files_ReadFile", readResult)
	readStructured := decodeMap(t, readResult.StructuredContent)
	if got := readStructured["content"]; got != "Hello from the Python file manager example.\n" {
		t.Fatalf("content = %v, want seeded example file content", got)
	}

	deleteResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "files_DeleteFile",
		Arguments: map[string]any{
			"filename": "example.txt",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(DeleteFile) failed: %v", err)
	}
	if deleteResult.IsError {
		t.Fatalf("DeleteFile returned tool error: %+v", deleteResult)
	}
	assertTextStructuredContentMatch(t, "files_DeleteFile", deleteResult)
	deleteStructured := decodeMap(t, deleteResult.StructuredContent)
	if got := deleteStructured["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
}

func runCRMExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "crm_ListUsers")
	findTool(t, tools.Tools, "crm_UpdateUser")

	listResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "crm_ListUsers",
		Arguments: map[string]any{
			"limit":        2,
			"requiredTags": []any{"premium"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(ListUsers) failed: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("ListUsers returned tool error: %+v", listResult)
	}
	assertTextStructuredContentMatch(t, "crm_ListUsers", listResult)
	listStructured := decodeMap(t, listResult.StructuredContent)
	users, ok := listStructured["users"].([]any)
	if !ok || len(users) != 1 {
		t.Fatalf("users = %T %v, want single filtered user", listStructured["users"], listStructured["users"])
	}
	user, ok := users[0].(map[string]any)
	if !ok {
		t.Fatalf("user[0] has type %T, want map[string]any", users[0])
	}

	updateResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "crm_UpdateUser",
		Arguments: map[string]any{
			"user": map[string]any{
				"id":           user["id"],
				"name":         "Alice Updated",
				"registeredAt": user["registeredAt"],
				"tags":         []any{"premium", "updated"},
			},
			"updateMask": "name,tags",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(UpdateUser) failed: %v", err)
	}
	if updateResult.IsError {
		t.Fatalf("UpdateUser returned tool error: %+v", updateResult)
	}
	assertTextStructuredContentMatch(t, "crm_UpdateUser", updateResult)
	updateStructured := decodeMap(t, updateResult.StructuredContent)
	updatedUser, ok := updateStructured["user"].(map[string]any)
	if !ok {
		t.Fatalf("updated user has type %T, want map[string]any", updateStructured["user"])
	}
	if got := updatedUser["name"]; got != "Alice Updated" {
		t.Fatalf("updated user.name = %v, want Alice Updated", got)
	}
}

func runStandaloneNotebookExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "notebook_CreateNote")
	findTool(t, tools.Tools, "notebook_SearchNotes")

	createResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "notebook_CreateNote",
		Arguments: map[string]any{
			"title":   "Ship Python support",
			"body":    "Verify the generated Python MCP bindings are pleasant to use.",
			"tags":    []any{"python", "mcp"},
			"dueDate": "2026-04-30",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(CreateNote) failed: %v", err)
	}
	if createResult.IsError {
		t.Fatalf("CreateNote returned tool error: %+v", createResult)
	}
	assertTextStructuredContentMatch(t, "notebook_CreateNote", createResult)
	createStructured := decodeMap(t, createResult.StructuredContent)
	note, ok := createStructured["note"].(map[string]any)
	if !ok {
		t.Fatalf("created note has type %T, want map[string]any", createStructured["note"])
	}
	if got := note["title"]; got != "Ship Python support" {
		t.Fatalf("created note.title = %v, want Ship Python support", got)
	}

	searchResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "notebook_SearchNotes",
		Arguments: map[string]any{
			"query": "python",
			"tags":  []any{"mcp"},
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(SearchNotes) failed: %v", err)
	}
	if searchResult.IsError {
		t.Fatalf("SearchNotes returned tool error: %+v", searchResult)
	}
	assertTextStructuredContentMatch(t, "notebook_SearchNotes", searchResult)
	searchStructured := decodeMap(t, searchResult.StructuredContent)
	notes, ok := searchStructured["notes"].([]any)
	if !ok || len(notes) != 1 {
		t.Fatalf("notes = %T %v, want one matching note", searchStructured["notes"], searchStructured["notes"])
	}
}

func runStandaloneTaskProtobufExample(t *testing.T, session *mcp.ClientSession) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "tasks_CreateTask")
	findTool(t, tools.Tools, "tasks_ListTasks")
	findTool(t, tools.Tools, "tasks_Health")

	createResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "tasks_CreateTask",
		Arguments: map[string]any{
			"title": "Ship protobuf handler docs",
			"tags":  []any{"python", "protobuf"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(CreateTask) failed: %v", err)
	}
	if createResult.IsError {
		t.Fatalf("CreateTask returned tool error: %+v", createResult)
	}
	assertTextStructuredContentMatch(t, "tasks_CreateTask", createResult)
	createStructured := decodeMap(t, createResult.StructuredContent)
	task, ok := createStructured["task"].(map[string]any)
	if !ok {
		t.Fatalf("created task has type %T, want map[string]any", createStructured["task"])
	}
	if got := task["title"]; got != "Ship protobuf handler docs" {
		t.Fatalf("created task.title = %v, want Ship protobuf handler docs", got)
	}
	if got := task["tags"]; !reflect.DeepEqual(got, []any{"python", "protobuf"}) {
		t.Fatalf("created task.tags = %v, want [python protobuf]", got)
	}

	listResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "tasks_ListTasks",
		Arguments: map[string]any{
			"tags":  []any{"protobuf"},
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(ListTasks) failed: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("ListTasks returned tool error: %+v", listResult)
	}
	assertTextStructuredContentMatch(t, "tasks_ListTasks", listResult)
	listStructured := decodeMap(t, listResult.StructuredContent)
	tasks, ok := listStructured["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("tasks = %T %v, want one matching task", listStructured["tasks"], listStructured["tasks"])
	}
	listedTask, ok := tasks[0].(map[string]any)
	if !ok {
		t.Fatalf("tasks[0] has type %T, want map[string]any", tasks[0])
	}
	if got := listedTask["title"]; got != "Ship protobuf handler docs" {
		t.Fatalf("listed task.title = %v, want Ship protobuf handler docs", got)
	}

	healthResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "tasks_Health",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(Health) failed: %v", err)
	}
	if healthResult.IsError {
		t.Fatalf("Health returned tool error: %+v", healthResult)
	}
	assertTextStructuredContentMatch(t, "tasks_Health", healthResult)
	healthStructured := decodeMap(t, healthResult.StructuredContent)
	if got := healthStructured["ok"]; got != true {
		t.Fatalf("health.ok = %v, want true", got)
	}
	if got := healthStructured["taskCount"]; got != 1.0 {
		t.Fatalf("health.taskCount = %v, want 1", got)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
}

func pythonExampleCommand(t *testing.T, root, script string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(pythontest.Python(t), script)
	cmd.Dir = root
	cmd.Env = pythontest.Env(t,
		"PYTHONPATH=",
		"PYTHONUNBUFFERED=1",
	)
	return cmd
}

func standalonePythonExampleCommand(t *testing.T, projectDir string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(pythontest.Python(t), "server.py")
	cmd.Dir = projectDir
	cmd.Env = pythontest.Env(t,
		"PYTHONPATH=",
		"PYTHONUNBUFFERED=1",
	)
	return cmd
}

func decodeMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON value: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal JSON value: %v", err)
	}

	return decoded
}

func assertTextStructuredContentMatch(t *testing.T, toolName string, result *mcp.CallToolResult) {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content items, want 1", toolName, len(result.Content))
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s content[0] has type %T, want *mcp.TextContent", toolName, result.Content[0])
	}

	var fromText map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &fromText); err != nil {
		t.Fatalf("decode text content for %s: %v", toolName, err)
	}

	fromStructured := decodeMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
	}
}

func findTool(t *testing.T, tools []*mcp.Tool, toolName string) *mcp.Tool {
	t.Helper()

	for _, tool := range tools {
		if tool != nil && tool.Name == toolName {
			return tool
		}
	}

	t.Fatalf("tool %q not found in tools/list", toolName)
	return nil
}
