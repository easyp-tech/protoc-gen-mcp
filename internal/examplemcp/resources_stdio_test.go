package examplemcp_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
	"github.com/easyp-tech/protoc-gen-mcp/internal/stdiotest"
)

func TestResourcesOverStdio(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), serveStdioEnv+"="+serveResourcesMode)
	stdiotest.RunResourcesPromptsContract(t, cmd)
}

func TestPythonResourcesOverStdio(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command(pythontest.Python(t), "-m", "internal.examplemcp.python_resources_server")
	cmd.Dir = root
	cmd.Env = pythontest.Env(t, "PYTHONPATH="+root, "PYTHONUNBUFFERED=1")
	stdiotest.RunResourcesPromptsContract(t, cmd)
}
