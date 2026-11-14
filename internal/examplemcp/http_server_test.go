package examplemcp_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/easyp-tech/protoc-gen-mcp/internal/httpcontract"
	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
	configv1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/config/v1"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// serveHTTPMode serves the OAuth-protected config/v1 fixture over
// Streamable HTTP for the shared HTTP contract.
const serveHTTPMode = "http"

type secureHandler struct{}

func (secureHandler) CheckAccess(_ context.Context, _ *configv1.CheckAccessRequest) (*configv1.CheckAccessResponse, error) {
	return &configv1.CheckAccessResponse{Allowed: true}, nil
}

func contractVerifier(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	scopes := httpcontract.DemoScopes(token)
	if scopes == nil {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{UserID: "contract", Scopes: scopes, Expiration: time.Now().Add(time.Hour)}, nil
}

func serveHTTP() int {
	server, err := configv1.NewServerConfigMCPServer(context.Background(), configv1.ServerConfigMCPHandlers{SecureAPI: secureHandler{}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create server: %v\n", err)
		return 1
	}
	handler, err := configv1.NewServerConfigMCPHTTPHandler(server, contractVerifier)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create HTTP handler: %v\n", err)
		return 1
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		return 1
	}
	fmt.Println(httpcontract.Listening("http://" + listener.Addr().String()))
	if err := http.Serve(listener, handler); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

func TestHTTPOAuthContract(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), serveStdioEnv+"="+serveHTTPMode)
	httpcontract.Run(t, cmd)
}

func TestPythonHTTPOAuthContract(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command(pythontest.Python(t), "-m", "internal.examplemcp.python_http_server")
	cmd.Dir = root
	cmd.Env = pythontest.Env(t, "PYTHONPATH="+root, "PYTHONUNBUFFERED=1")
	httpcontract.Run(t, cmd)
}

func TestJavaHTTPOAuthContract(t *testing.T) {
	root := repoRoot(t)
	ensureJVMExamplesInstalled(t, root)
	cmd := installedJVMServerCommand(root, "java-server")
	cmd.Args = append(cmd.Args, "http")
	httpcontract.Run(t, cmd)
}
