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

// serveResourcesMode serves the resources and prompts fixture server instead
// of the tool example server.
const serveResourcesMode = "resources"

func TestMain(m *testing.M) {
	switch os.Getenv(serveStdioEnv) {
	case "1":
		os.Exit(serveOverStdio(examplemcp.NewServer))
	case serveHTTPMode:
		os.Exit(serveHTTP())
	case serveResourcesMode:
		os.Exit(serveOverStdio(func() (*mcp.Server, error) {
			return examplemcp.NewResourcesServer(context.Background())
		}))
	}
	os.Exit(m.Run())
}

func serveOverStdio(newServer func() (*mcp.Server, error)) int {
	server, err := newServer()
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
