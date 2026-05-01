package pythontest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	cacheVersion = "v1"
	probeScript  = "import anyio, jsonschema, mcp\nimport google.protobuf\nmajor = int(google.protobuf.__version__.split('.', 1)[0])\nassert 6 <= major < 7, google.protobuf.__version__\n"
)

var (
	pythonOnce sync.Once
	pythonPath string
	venvDir    string
	setupErr   error
	skipReason string
)

// Python returns an isolated interpreter with the Python runtime dependencies
// required by generated MCP bindings. It deliberately avoids global
// site-packages so Go tests are not coupled to the developer's local protobuf
// installation.
func Python(t testing.TB) string {
	t.Helper()

	pythonOnce.Do(func() {
		pythonPath, venvDir, setupErr = ensurePython()
		if setupErr != nil && strings.HasPrefix(setupErr.Error(), "skip: ") {
			skipReason = strings.TrimPrefix(setupErr.Error(), "skip: ")
			setupErr = nil
		}
	})
	if skipReason != "" {
		t.Skip(skipReason)
	}
	if setupErr != nil {
		t.Fatalf("prepare isolated Python test runtime: %v", setupErr)
	}
	return pythonPath
}

// Command creates an exec.Cmd that runs inside the isolated Python test
// environment.
func Command(t testing.TB, args ...string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(Python(t), args...)
	cmd.Env = Env(t)
	return cmd
}

// Env returns process environment variables for commands that should use the
// isolated Python test runtime.
func Env(t testing.TB, overrides ...string) []string {
	t.Helper()

	Python(t)
	binDir := filepath.Dir(pythonPath)
	env := mergeEnv(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PYTHONNOUSERSITE=1",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_INPUT=1",
	)
	if venvDir != "" {
		env = mergeEnv(env, "VIRTUAL_ENV="+venvDir)
	}
	return mergeEnv(env, overrides...)
}

func ensurePython() (string, string, error) {
	if explicit := os.Getenv("PROTOC_GEN_MCP_TEST_PYTHON"); explicit != "" {
		if err := probePython(explicit); err != nil {
			return "", "", fmt.Errorf("PROTOC_GEN_MCP_TEST_PYTHON=%q is missing required packages: %w", explicit, err)
		}
		return explicit, "", nil
	}

	basePython, err := findBasePython()
	if err != nil {
		return "", "", err
	}
	version, err := pythonVersion(basePython)
	if err != nil {
		return "", "", err
	}
	cacheRoot, err := pythonCacheRoot()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(cacheRoot, "python-"+runtime.GOOS+"-"+runtime.GOARCH+"-"+version+"-"+cacheVersion)
	python := venvPython(dir)
	ready := filepath.Join(dir, ".ready")

	if isReady(python, ready) {
		return python, dir, nil
	}
	if err := withSetupLock(dir+".lock", func() error {
		if isReady(python, ready) {
			return nil
		}
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove stale venv: %w", err)
		}
		if err := run(basePython, "-m", "venv", dir); err != nil {
			return fmt.Errorf("create venv: %w", err)
		}
		if err := run(python, "-m", "pip", "install", "--upgrade",
			"mcp>=1.27,<2",
			"protobuf>=6,<7",
			"jsonschema>=4,<5",
			"anyio>=4,<5",
		); err != nil {
			return fmt.Errorf("install Python test dependencies: %w", err)
		}
		if err := probePython(python); err != nil {
			return fmt.Errorf("probe installed Python test dependencies: %w", err)
		}
		return os.WriteFile(ready, []byte(time.Now().Format(time.RFC3339Nano)+"\n"), 0o644)
	}); err != nil {
		return "", "", err
	}
	return python, dir, nil
}

func findBasePython() (string, error) {
	if explicit := os.Getenv("PROTOC_GEN_MCP_BASE_PYTHON"); explicit != "" {
		return explicit, nil
	}
	if path, err := exec.LookPath("python3"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("python"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("skip: python3/python not found in PATH")
}

func pythonVersion(python string) (string, error) {
	cmd := exec.Command(python, "-c", "import sys\nassert sys.version_info >= (3, 10), sys.version\nprint(f'{sys.version_info.major}.{sys.version_info.minor}')\n")
	cmd.Env = mergeEnv(os.Environ(), "PYTHONNOUSERSITE=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("python %q must be 3.10 or newer: %w\n%s", python, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func pythonCacheRoot() (string, error) {
	if explicit := os.Getenv("PROTOC_GEN_MCP_TEST_CACHE"); explicit != "" {
		return explicit, os.MkdirAll(explicit, 0o755)
	}
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}
	root = filepath.Join(root, "protoc-gen-mcp", "python-test")
	return root, os.MkdirAll(root, 0o755)
}

func venvPython(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "Scripts", "python.exe")
	}
	return filepath.Join(dir, "bin", "python")
}

func isReady(python, ready string) bool {
	if _, err := os.Stat(ready); err != nil {
		return false
	}
	return probePython(python) == nil
}

func probePython(python string) error {
	return run(python, "-c", probeScript)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = mergeEnv(os.Environ(),
		"PYTHONNOUSERSITE=1",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_INPUT=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, output)
	}
	return nil
}

func withSetupLock(lockPath string, fn func() error) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = file.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !os.IsExist(err) {
			return fmt.Errorf("create setup lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for Python test runtime setup lock %s", lockPath)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func mergeEnv(base []string, overrides ...string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, item := range append(base, overrides...) {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}

	merged := make([]string, 0, len(values))
	for _, key := range order {
		merged = append(merged, key+"="+values[key])
	}
	return merged
}
