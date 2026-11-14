package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// typeScriptFixtureProtos are the shared fixtures every target must compile.
var typeScriptFixtureProtos = []string{
	"internal/testproto/config/v1/server_config.proto",
	"internal/testproto/example/v1/example.proto",
	"internal/testproto/prompts/v1/prompts.proto",
	"internal/testproto/resources/v1/resources.proto",
}

// TestTypeScriptFixturesCompileWithProtobufES compiles the generated
// TypeScript sidecars of every shared fixture against real protoc-gen-es
// output under the strict NodeNext settings users get.
// generateTypeScriptFixtureProject writes real protoc-gen-es output and the
// protoc-gen-mcp TypeScript sidecars of protos into a temporary project under
// the harness, so it resolves the harness node_modules.
func generateTypeScriptFixtureProject(t *testing.T, harness string, protos []string) string {
	t.Helper()
	protocGenES := filepath.Join(harness, "node_modules", ".bin", "protoc-gen-es")
	if _, err := os.Stat(protocGenES); err != nil {
		t.Fatalf("protoc-gen-es is required; run npm ci in internal/codegen/testdata/node: %v", err)
	}
	resolver := &protocompile.SourceResolver{ImportPaths: []string{repoRoot(t)}}

	project, err := os.MkdirTemp(harness, ".tmp-typescript-fixtures-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript project: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(project) })

	esRequest := newCompiledCodeGeneratorRequest(t, append([]string{"mcp/options/v1/options.proto"}, protos...), resolver)
	esRequest.Parameter = proto.String("target=ts,import_extension=js")
	payload, err := proto.Marshal(esRequest)
	if err != nil {
		t.Fatalf("marshal protoc-gen-es request: %v", err)
	}
	run := exec.Command(protocGenES)
	run.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	run.Stderr = &stderr
	output, err := run.Output()
	if err != nil {
		t.Fatalf("protoc-gen-es: %v\n%s", err, stderr.String())
	}
	var esResponse pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(output, &esResponse); err != nil {
		t.Fatalf("decode protoc-gen-es response: %v", err)
	}
	if esResponse.GetError() != "" {
		t.Fatalf("protoc-gen-es error: %s", esResponse.GetError())
	}
	files := esResponse.GetFile()

	plugin := newCompiledProtogenPlugin(t, protos, resolver)
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript fixtures: %v", err)
	}
	mcpResponse := plugin.Response()
	if mcpResponse.GetError() != "" {
		t.Fatalf("protoc-gen-mcp error: %s", mcpResponse.GetError())
	}
	if len(mcpResponse.GetFile()) != len(protos) {
		t.Fatalf("protoc-gen-mcp produced %d TypeScript files, want %d", len(mcpResponse.GetFile()), len(protos))
	}
	files = append(files, mcpResponse.GetFile()...)

	for _, file := range files {
		path := filepath.Join(project, filepath.FromSlash(file.GetName()))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		writeTypeScriptFixtureFile(t, path, []byte(file.GetContent()))
	}
	return project
}

func TestTypeScriptFixturesCompileWithProtobufES(t *testing.T) {
	harness := requireTypeScriptSDKSpike(t)
	project := generateTypeScriptFixtureProject(t, harness, typeScriptFixtureProtos)
	writeTypeScriptFixtureFile(t, filepath.Join(project, "tsconfig.json"), []byte(`{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "verbatimModuleSyntax": true,
    "noEmit": true,
    "types": ["node"]
  },
  "include": ["**/*.ts"]
}
`))

	compile := exec.Command("npm", "exec", "--prefix", harness, "--", "tsc", "-p", filepath.Join(project, "tsconfig.json"))
	compile.Dir = harness
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated TypeScript fixtures do not compile:\n%s", out)
	}
}
