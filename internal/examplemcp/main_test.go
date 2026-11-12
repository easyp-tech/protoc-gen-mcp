package examplemcp_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serveStdioEnv makes the test binary act as the example MCP server, so stdio
// tests can run a real server process without a separate command package.
const serveStdioEnv = "EXAMPLEMCP_SERVE_STDIO"

func TestMain(m *testing.M) {
	if os.Getenv(serveStdioEnv) == "1" {
		os.Exit(serveExampleOverStdio())
	}
	os.Exit(m.Run())
}

func serveExampleOverStdio() int {
	server, err := examplemcp.NewServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create example server: %v\n", err)
		return 1
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "run example server: %v\n", err)
		return 1
	}
	return 0
}
