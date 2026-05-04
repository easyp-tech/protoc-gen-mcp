package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypeScriptGeneratedPublicAPICompilesUnderNodeNext(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skipf("node is required for TypeScript compile smoke: %v", err)
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skipf("npm is required for TypeScript compile smoke: %v", err)
	}

	spikeDir, err := filepath.Abs(filepath.Join("..", "..", "examples", "node", "sdk-spike"))
	if err != nil {
		t.Fatalf("resolve examples/node/sdk-spike: %v", err)
	}
	if _, err := os.Stat(filepath.Join(spikeDir, "package.json")); err != nil {
		t.Fatalf("examples/node/sdk-spike package.json is required: %v", err)
	}
	if _, err := os.Stat(filepath.Join(spikeDir, "node_modules", ".bin", "tsc")); err != nil {
		t.Fatalf("local TypeScript compiler is required; run npm ci in examples/node/sdk-spike: %v", err)
	}

	plugin := newTempProtogenPlugin(t, map[string]string{
		"compile/v1/public-api.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package compile.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/compile;compilev1";`,
			`import "mcp/options/v1/options.proto";`,
			`service PublicAPI {`,
			`  option (mcp.options.v1.service) = { namespace: "compile.default" };`,
			`  rpc Render(RenderRequest) returns (RenderResponse) {`,
			`    option (mcp.options.v1.method) = { name: "render.output" };`,
			`  }`,
			`  rpc RenderNested(Render.NestedRequest) returns (Render.NestedResponse);`,
			`}`,
			`message Render {`,
			`  message NestedRequest { string label = 1; }`,
			`  message NestedResponse { string output = 1; }`,
			`}`,
			`message RenderRequest { string label = 1; }`,
			`message RenderResponse { string output = 1; }`,
			``,
		}, "\n"),
	}, "compile/v1/public-api.proto")
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript: %v", err)
	}

	tempProject, err := os.MkdirTemp(spikeDir, ".tmp-typescript-compile-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript compile project under sdk-spike: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempProject); err != nil {
			t.Errorf("remove temporary TypeScript compile project: %v", err)
		}
	})

	sourceDir := filepath.Join(tempProject, "compile", "v1")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create compile fixture source dir: %v", err)
	}

	generatedSidecar := generatedFileContent(t, plugin, "compile/v1/public_api_mcp.ts")
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "public_api_mcp.ts"), generatedSidecar)
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "public_api_pb.ts"), []byte(protobufESCompileFixtureModule()))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "usage.ts"), []byte(typeScriptPublicAPIUsageFixture()))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "tsconfig.json"), []byte(typeScriptCompileFixtureTSConfig()))

	cmd := exec.Command("npm", "exec", "--prefix", spikeDir, "--", "tsc", "-p", filepath.Join(tempProject, "tsconfig.json"), "--noEmit")
	cmd.Dir = spikeDir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript public API failed NodeNext compile via npm exec:\n%s", string(output))
	}
}

func writeTypeScriptFixtureFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.WriteFile(path, bytes.TrimPrefix(contents, []byte("\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func protobufESCompileFixtureModule() string {
	return `
import type { Message } from "@bufbuild/protobuf";

export type RenderRequest = Message<"compile.v1.RenderRequest"> & {
  label: string;
};

export type RenderResponse = Message<"compile.v1.RenderResponse"> & {
  output: string;
};

export type Render_NestedRequest = Message<"compile.v1.Render.NestedRequest"> & {
  label: string;
};

export type Render_NestedResponse = Message<"compile.v1.Render.NestedResponse"> & {
  output: string;
};

export const RenderRequestSchema = {
  typeName: "compile.v1.RenderRequest",
};

export const RenderResponseSchema = {
  typeName: "compile.v1.RenderResponse",
};

export const Render_NestedRequestSchema = {
  typeName: "compile.v1.Render.NestedRequest",
};

export const Render_NestedResponseSchema = {
  typeName: "compile.v1.Render.NestedResponse",
};

export const file_compile_v1_public_api = {
  proto: {
    name: "compile/v1/public-api.proto",
  },
};
`
}

func typeScriptPublicAPIUsageFixture() string {
	return `
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { registerPublicAPITools, type PublicAPIToolHandler } from "./public_api_mcp.js";
import type {
  Render_NestedRequest,
  Render_NestedResponse,
  RenderRequest,
  RenderResponse,
} from "./public_api_pb.js";

const handler: PublicAPIToolHandler = {
  render(_ctx, request: RenderRequest): RenderResponse {
    return {
      $typeName: "compile.v1.RenderResponse",
      output: request.label.toUpperCase(),
    };
  },
  renderNested(_ctx, request: Render_NestedRequest): Render_NestedResponse {
    return {
      $typeName: "compile.v1.Render.NestedResponse",
      output: request.label.toLowerCase(),
    };
  },
};

const server = new Server(
  { name: "generated-public-api-compile", version: "0.0.0" },
  { capabilities: { tools: {} } },
);

registerPublicAPITools(server, handler, "");

void server;
`
}

func typeScriptCompileFixtureTSConfig() string {
	return `
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "verbatimModuleSyntax": true
  },
  "include": [
    "compile/v1/public_api_mcp.ts",
    "compile/v1/public_api_pb.ts",
    "compile/v1/usage.ts"
  ]
}
`
}
