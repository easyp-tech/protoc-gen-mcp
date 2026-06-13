package examples_test

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
)

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// stdioClient communicates with a subprocess MCP server over stdin/stdout JSON-RPC.
type stdioClient struct {
	t       *testing.T
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	nextID  int
}

func newStdioClient(t *testing.T, cmd *exec.Cmd) *stdioClient {
	t.Helper()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}

	t.Cleanup(func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	})

	return &stdioClient{
		t:       t,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
	}
}

func (c *stdioClient) call(method string, params any) jsonrpcResponse {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	id := c.nextID
	c.nextID++
	idBytes, _ := json.Marshal(id)

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		c.t.Fatalf("write request: %v", err)
	}

	if !c.scanner.Scan() {
		c.t.Fatalf("no response for %s (scanner error: %v)", method, c.scanner.Err())
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		c.t.Fatalf("unmarshal response for %s: %v (raw: %s)", method, err, c.scanner.Bytes())
	}
	return resp
}

func (c *stdioClient) notify(method string, params any) {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal notification: %v", err)
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		c.t.Fatalf("write notification: %v", err)
	}
}

func (c *stdioClient) initialize() {
	c.t.Helper()
	resp := c.call("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test-client", "version": "v0.0.1"},
	})
	if resp.Error != nil {
		c.t.Fatalf("initialize failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	c.notify("notifications/initialized", map[string]any{})
}

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

type callToolResult struct {
	Content           []json.RawMessage `json:"content,omitempty"`
	StructuredContent json.RawMessage   `json:"structuredContent,omitempty"`
	IsError           bool              `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *stdioClient) listTools() toolsListResult {
	c.t.Helper()
	resp := c.call("tools/list", map[string]any{})
	if resp.Error != nil {
		c.t.Fatalf("tools/list failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	var result toolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/list result: %v", err)
	}
	return result
}

func (c *stdioClient) callTool(name string, arguments map[string]any) (callToolResult, *jsonrpcError) {
	c.t.Helper()
	resp := c.call("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if resp.Error != nil {
		return callToolResult{}, resp.Error
	}
	var result callToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/call result: %v", err)
	}
	return result, nil
}

func TestPythonExamplesOverStdio(t *testing.T) {
	root := repoRoot(t)
	examples := []struct {
		name string
		path string
		run  func(t *testing.T, client *stdioClient)
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
			cmd := pythonExampleCommand(t, root, tc.path)
			client := newStdioClient(t, cmd)
			client.initialize()

			tc.run(t, client)
		})
	}
}

func TestStandalonePythonExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/5_python_standalone")

	cmd := standalonePythonExampleCommand(t, projectDir)
	client := newStdioClient(t, cmd)
	client.initialize()

	runStandaloneNotebookExample(t, client)
}

func TestStandalonePythonProtobufExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/10_python_protobuf_standalone")

	cmd := standalonePythonExampleCommand(t, projectDir)
	client := newStdioClient(t, cmd)
	client.initialize()

	runStandaloneTaskProtobufExample(t, client)
}

func runHelloWorldExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "greeter_SayHello")

	result, rpcErr := client.callTool("greeter_SayHello", map[string]any{
		"name": "Alice",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(SayHello) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

func runWeatherExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "weather_GetCurrentWeather")

	result, rpcErr := client.callTool("weather_GetCurrentWeather", map[string]any{
		"city": "London",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(GetCurrentWeather) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

func runFileManagerExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "files_ReadFile")
	findTool(t, tools.Tools, "files_DeleteFile")

	readResult, rpcErr := client.callTool("files_ReadFile", map[string]any{
		"filename": "example.txt",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(ReadFile) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if readResult.IsError {
		t.Fatalf("ReadFile returned tool error: %+v", readResult)
	}
	assertTextStructuredContentMatch(t, "files_ReadFile", readResult)
	readStructured := decodeMap(t, readResult.StructuredContent)
	if got := readStructured["content"]; got != "Hello from the Python file manager example.\n" {
		t.Fatalf("content = %v, want seeded example file content", got)
	}

	deleteResult, rpcErr := client.callTool("files_DeleteFile", map[string]any{
		"filename": "example.txt",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(DeleteFile) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

func runCRMExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "crm_ListUsers")
	findTool(t, tools.Tools, "crm_UpdateUser")

	listResult, rpcErr := client.callTool("crm_ListUsers", map[string]any{
		"limit":        2,
		"requiredTags": []any{"premium"},
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(ListUsers) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

	updateResult, rpcErr := client.callTool("crm_UpdateUser", map[string]any{
		"user": map[string]any{
			"id":           user["id"],
			"name":         "Alice Updated",
			"registeredAt": user["registeredAt"],
			"tags":         []any{"premium", "updated"},
		},
		"updateMask": "name,tags",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(UpdateUser) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

func runStandaloneNotebookExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "notebook_CreateNote")
	findTool(t, tools.Tools, "notebook_SearchNotes")

	createResult, rpcErr := client.callTool("notebook_CreateNote", map[string]any{
		"title":   "Ship Python support",
		"body":    "Verify the generated Python MCP bindings are pleasant to use.",
		"tags":    []any{"python", "mcp"},
		"dueDate": "2026-04-30",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(CreateNote) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

	searchResult, rpcErr := client.callTool("notebook_SearchNotes", map[string]any{
		"query": "python",
		"tags":  []any{"mcp"},
		"limit": 5,
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(SearchNotes) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

func runStandaloneTaskProtobufExample(t *testing.T, client *stdioClient) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "tasks_CreateTask")
	findTool(t, tools.Tools, "tasks_ListTasks")
	findTool(t, tools.Tools, "tasks_Health")

	createResult, rpcErr := client.callTool("tasks_CreateTask", map[string]any{
		"title": "Ship protobuf handler docs",
		"tags":  []any{"python", "protobuf"},
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(CreateTask) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

	listResult, rpcErr := client.callTool("tasks_ListTasks", map[string]any{
		"tags":  []any{"protobuf"},
		"limit": 5,
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(ListTasks) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

	healthResult, rpcErr := client.callTool("tasks_Health", map[string]any{})
	if rpcErr != nil {
		t.Fatalf("CallTool(Health) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
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

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case json.RawMessage:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal JSON value: %v", err)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal JSON value: %v", err)
	}

	return decoded
}

func assertTextStructuredContentMatch(t *testing.T, toolName string, result callToolResult) {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content items, want 1", toolName, len(result.Content))
	}

	var tc textContent
	if err := json.Unmarshal(result.Content[0], &tc); err != nil {
		t.Fatalf("unmarshal text content for %s: %v", toolName, err)
	}

	var fromText map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &fromText); err != nil {
		t.Fatalf("decode text content for %s: %v", toolName, err)
	}

	fromStructured := decodeMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
	}
}

func findTool(t *testing.T, tools []toolInfo, toolName string) toolInfo {
	t.Helper()

	for _, tool := range tools {
		if tool.Name == toolName {
			return tool
		}
	}

	t.Fatalf("tool %q not found in tools/list", toolName)
	return toolInfo{}
}
