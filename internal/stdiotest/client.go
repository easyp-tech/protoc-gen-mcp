// Package stdiotest drives generated MCP servers over stdio JSON-RPC so the
// same behavioral contract can be checked against every target language.
package stdiotest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// responseTimeout bounds every request so a server that never answers fails
// the test with its stderr instead of hanging until the go test timeout.
const responseTimeout = 90 * time.Second

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Client talks newline-delimited JSON-RPC to a server subprocess.
type Client struct {
	t       *testing.T
	stdin   io.WriteCloser
	lines   chan []byte
	stderr  *syncBuffer
	nextID  int
	initRes json.RawMessage
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// NewClient starts cmd and stops it when the test finishes.
func NewClient(t *testing.T, cmd *exec.Cmd) *Client {
	t.Helper()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &syncBuffer{}
	if cmd.Stderr == nil {
		cmd.Stderr = stderr
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	lines := make(chan []byte, 16)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			lines <- append([]byte(nil), scanner.Bytes()...)
		}
	}()
	return &Client{t: t, stdin: stdin, lines: lines, stderr: stderr, nextID: 1}
}

// Call sends a request and returns the matching response, skipping any
// server-initiated messages in between.
func (c *Client) Call(method string, params any) Response {
	c.t.Helper()

	id := c.nextID
	c.nextID++
	idBytes, _ := json.Marshal(id)
	c.write(request{JSONRPC: "2.0", ID: idBytes, Method: method, Params: c.marshal(params)})

	timeout := time.After(responseTimeout)
	for {
		var line []byte
		select {
		case received, ok := <-c.lines:
			if !ok {
				c.t.Fatalf("server closed stdout before answering %s; stderr:\n%s", method, c.stderr.String())
			}
			line = received
		case <-timeout:
			c.t.Fatalf("no response for %s within %s; stderr:\n%s", method, responseTimeout, c.stderr.String())
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			c.t.Fatalf("unmarshal response for %s: %v (raw: %s)", method, err, line)
		}
		if string(resp.ID) == string(idBytes) {
			return resp
		}
	}
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) {
	c.t.Helper()
	c.write(request{JSONRPC: "2.0", Method: method, Params: c.marshal(params)})
}

// Initialize performs the MCP handshake and keeps the initialize result.
func (c *Client) Initialize() {
	c.t.Helper()
	resp := c.Call("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test-client", "version": "v0.0.1"},
	})
	if resp.Error != nil {
		c.t.Fatalf("initialize failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	c.initRes = resp.Result
	c.Notify("notifications/initialized", map[string]any{})
}

// InitializeResult returns the raw initialize result.
func (c *Client) InitializeResult() json.RawMessage {
	return c.initRes
}

// Result calls method, fails the test on a JSON-RPC error and decodes the result into out.
func (c *Client) Result(method string, params any, out any) {
	c.t.Helper()
	resp := c.Call(method, params)
	if resp.Error != nil {
		c.t.Fatalf("%s failed: code=%d msg=%s", method, resp.Error.Code, resp.Error.Message)
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		c.t.Fatalf("unmarshal %s result: %v (raw: %s)", method, err, resp.Result)
	}
}

func (c *Client) marshal(params any) json.RawMessage {
	c.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}
	return data
}

func (c *Client) write(req request) {
	c.t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		c.t.Fatalf("write %s: %v", req.Method, err)
	}
}
