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

func TestStandalonePythonExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/5_python_standalone")

	cmd := standalonePythonExampleCommand(t, projectDir)
	client := newStdioClient(t, cmd)
	client.initialize()

	runStandaloneNotebookExample(t, client)
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
	assertNotebookResourcesAndPrompts(t, client, note)

	// DeleteNote needs notebook:write; stdio carries no token, so the call is denied.
	denied, rpcErr := client.callTool("notebook_DeleteNote", map[string]any{"id": note["id"]})
	if rpcErr != nil || !denied.IsError {
		t.Fatalf("DeleteNote over stdio = %+v (rpc %v), want a permission-denied tool error", denied, rpcErr)
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

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
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
