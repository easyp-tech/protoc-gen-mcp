package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
	"google.golang.org/protobuf/compiler/protogen"
)

// reservedParamsTemplate uses names that are reserved in at least one target
// language or that used to clash with identifiers in the generated Go handler.
const reservedParamsTemplate = "items://{type}/{ctx}/{result}/{params}/{class}/{function}/{fun}/{default}/{self}/{_}/{type_}"

var reservedParams = []string{"type", "ctx", "result", "params", "class", "function", "fun", "default", "self", "_", "type_"}

func newReservedParamsPlugin(t *testing.T) *protogen.Plugin {
	t.Helper()
	return newTempProtogenPlugin(t, map[string]string{
		"test/v1/items.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "example.com/items/testv1;testv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message Item {`,
			`  option (mcp.options.v1.resource) = { uri_template: "` + reservedParamsTemplate + `" description: "Item" };`,
			`  string name = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/items.proto")
}

func reservedParamModels() []ResourceParamModel {
	params := make([]ResourceParamModel, 0, len(reservedParams))
	for _, name := range reservedParams {
		params = append(params, ResourceParamModel{Name: name})
	}
	return params
}

func TestParamIdentifiersEscapeReservedNames(t *testing.T) {
	params := reservedParamModels()
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{"go", goParamIdentifiers(params),
			[]string{"type__", "ctx_", "result", "params", "class", "function", "fun", "default_", "self", "_", "type_"}},
		{"python", pythonParamIdentifiers(params),
			[]string{"type", "ctx", "result", "params", "class_", "function", "fun", "default", "self_", "_", "type_"}},
		{"typescript", typescriptParamIdentifiers(params),
			[]string{"type", "ctx", "result", "params", "class_", "function_", "fun", "default_", "self", "_", "type_"}},
		{"kotlin", kotlinParamIdentifiers(params),
			[]string{"type", "ctx", "result", "params", "class_", "function", "fun_", "default", "self", "param_", "type_"}},
		{"java", javaParamIdentifiers(params),
			[]string{"type", "ctx", "result", "params", "class_", "function", "fun", "default_", "self", "__", "type_"}},
	}
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.got, tt.want) {
			t.Errorf("%s identifiers = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestCollectResources_TemplateWithRepeatedParam(t *testing.T) {
	_, err := collectResourceFixture(t, ` name: "s" uri_template: "server://{id}/{id}" `)
	if err == nil || !strings.Contains(err.Error(), `repeats parameter "id"`) {
		t.Fatalf("error = %v, want repeated parameter error", err)
	}
}

func TestGoResourceWithReservedParamNamesCompiles(t *testing.T) {
	plugin := newReservedParamsPlugin(t)
	if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	generated := generatedFileContent(t, plugin, "test/v1/items.mcp.go")
	if !strings.Contains(string(generated), `impl.ReadItem(ctx, params["type"], params["ctx"],`) {
		t.Fatalf("Read call must pass template values by their wire names:\n%s", generated)
	}

	// Type-check the output in a throwaway module that resolves mcpruntime
	// from this repository and stubs the protobuf message.
	root := repoRoot(t)
	dir := t.TempDir()
	goMod := "module example.com/items/testv1\n\ngo 1.26\n\nrequire github.com/easyp-tech/protoc-gen-mcp v0.0.0\n\nreplace github.com/easyp-tech/protoc-gen-mcp => " + root + "\n"
	goSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatalf("read go.sum: %v", err)
	}
	stub := "package testv1\n\nimport \"google.golang.org/protobuf/reflect/protoreflect\"\n\ntype Item struct{}\n\nfunc (*Item) ProtoReflect() protoreflect.Message { return nil }\n"
	for name, content := range map[string][]byte{
		"go.mod":       []byte(goMod),
		"go.sum":       goSum,
		"stub.go":      []byte(stub),
		"items.mcp.go": generated,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go does not compile: %v\n%s\n%s", err, out, generated)
	}
}

func TestPythonResourceWithReservedParamNamesParses(t *testing.T) {
	plugin := newReservedParamsPlugin(t)
	if err := Generate(plugin, Options{Language: LanguagePython, PythonRuntime: PythonRuntimeGoogleProtobuf}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "items_mcp.py")
	if err := os.WriteFile(path, generatedFileContent(t, plugin, "test/v1/items_mcp.py"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := pythontest.Command(t, "-c", "import ast, sys; ast.parse(open(sys.argv[1]).read())", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Python does not parse: %v\n%s", err, out)
	}
}

func TestJVMAndTypeScriptResourceSignaturesEscapeReservedNames(t *testing.T) {
	tests := []struct {
		lang Language
		path string
		want string
	}{
		{LanguageTypeScript, "test/v1/items_mcp.ts",
			"readItem(type: string, ctx: string, result: string, params: string, class_: string, function_: string, fun: string, default_: string, self: string, _: string, type_: string)"},
		{LanguageKotlin, "test/v1/items_mcp.kt",
			"suspend fun readItem(type: String, ctx: String, result: String, params: String, class_: String, function: String, fun_: String, default: String, self: String, param_: String, type_: String)"},
		{LanguageJava, javaSidecarOutputPath(jvmGeneratedFilenamePrefixForProtoPath("test/v1/items.proto")),
			"readItem(String type, String ctx, String result, String params, String class_, String function, String fun, String default_, String self, String __, String type_)"},
	}
	for _, tt := range tests {
		plugin := newReservedParamsPlugin(t)
		if err := Generate(plugin, Options{Language: tt.lang}); err != nil {
			t.Fatalf("Generate %s: %v", tt.lang, err)
		}
		if got := string(generatedFileContent(t, plugin, tt.path)); !strings.Contains(got, tt.want) {
			t.Errorf("%s output is missing %q:\n%s", tt.lang, tt.want, got)
		}
	}
}

func TestSwiftResourceSignatureEscapesReservedNames(t *testing.T) {
	plugin := newReservedParamsPlugin(t)
	if err := Generate(plugin, Options{Language: LanguageSwift}); err != nil {
		t.Fatalf("Generate swift: %v", err)
	}
	got := string(generatedFileContent(t, plugin, "test/v1/items.mcp.swift"))
	start := strings.Index(got, "func readItem(")
	if start < 0 {
		t.Fatalf("Swift output has no readItem:\n%s", got)
	}
	line := got[start : start+strings.Index(got[start:], "\n")]
	for _, label := range []string{"class_: String", "default_: String", "self_: String", "params_: String"} {
		if !strings.Contains(line, label) {
			t.Errorf("readItem signature %q lacks %q", line, label)
		}
	}
}
