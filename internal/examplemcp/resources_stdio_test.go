package examplemcp_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/stdiotest"
)

func TestResourcesOverStdio(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), serveStdioEnv+"="+serveResourcesMode)
	stdiotest.RunResourcesPromptsContract(t, cmd)
}
