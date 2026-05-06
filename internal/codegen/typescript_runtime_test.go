package codegen

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile/protoutil"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func TestTypeScriptRuntimeContract(t *testing.T) {
	spikeDir := requireTypeScriptSDKSpike(t)
	const protoPath = "runtime/v1/runtime-contract.proto"
	plugin := newTempProtogenPlugin(t, map[string]string{
		protoPath: strings.Join([]string{
			`syntax = "proto3";`,
			`package runtime.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/runtime;runtimev1";`,
			`service RuntimeAPI {`,
			`  rpc Echo(EchoRequest) returns (EchoResponse);`,
			`}`,
			`message EchoRequest {`,
			`  string label = 1;`,
			`  int32 count = 2;`,
			`  bytes payload = 3;`,
			`}`,
			`message EchoResponse {`,
			`  string label = 1;`,
			`  int32 count = 2;`,
			`  bytes payload = 3;`,
			`}`,
			``,
		}, "\n"),
	}, protoPath)
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript: %v", err)
	}

	tempProject, err := os.MkdirTemp(spikeDir, ".tmp-typescript-runtime-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript runtime project under sdk-spike: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempProject); err != nil {
			t.Errorf("remove temporary TypeScript runtime project: %v", err)
		}
	})

	sourceDir := filepath.Join(tempProject, "runtime", "v1")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create runtime fixture source dir: %v", err)
	}

	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "runtime_contract_mcp.ts"), generatedFileContent(t, plugin, "runtime/v1/runtime_contract_mcp.ts"))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "runtime_contract_pb.ts"), []byte(protobufESRuntimeFixtureModule(t, plugin, protoPath)))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "runtime.ts"), []byte(typeScriptRuntimeContractFixture()))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "tsconfig.json"), []byte(typeScriptRuntimeFixtureTSConfig()))

	compile := exec.Command("npm", "exec", "--prefix", spikeDir, "--", "tsc", "-p", filepath.Join(tempProject, "tsconfig.json"))
	compile.Dir = spikeDir
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	compileOutput, err := compile.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript runtime contract failed NodeNext compile via npm exec:\n%s", string(compileOutput))
	}

	run := exec.Command("node", filepath.Join(tempProject, "dist", "runtime", "v1", "runtime.js"))
	run.Dir = spikeDir
	run.Env = append(os.Environ(), "NO_COLOR=1")
	runOutput, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript runtime contract failed under node:\n%s", string(runOutput))
	}
}

func protobufESRuntimeFixtureModule(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	descriptorBase64 := protobufESFileDescriptorBase64(t, plugin, protoPath)
	return fmt.Sprintf(`
import type { Message } from "@bufbuild/protobuf";
import { fileDesc, messageDesc, type GenFile, type GenMessage } from "@bufbuild/protobuf/codegenv2";

export const file_runtime_v1_runtime_contract: GenFile = fileDesc(%q);

export type EchoRequest = Message<"runtime.v1.EchoRequest"> & {
  label: string;
  count: number;
  payload: Uint8Array;
};

export type EchoResponse = Message<"runtime.v1.EchoResponse"> & {
  label: string;
  count: number;
  payload: Uint8Array;
};

export const EchoRequestSchema: GenMessage<EchoRequest> = messageDesc(file_runtime_v1_runtime_contract, 0);
export const EchoResponseSchema: GenMessage<EchoResponse> = messageDesc(file_runtime_v1_runtime_contract, 1);
`, descriptorBase64)
}

func protobufESFileDescriptorBase64(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	var descriptorBase64 string
	for _, file := range plugin.Files {
		if file.Desc.Path() != protoPath {
			continue
		}
		descriptorBytes, err := proto.Marshal(protoutil.ProtoFromFileDescriptor(file.Desc))
		if err != nil {
			t.Fatalf("marshal %s descriptor proto: %v", protoPath, err)
		}
		descriptorBase64 = base64.StdEncoding.EncodeToString(descriptorBytes)
		break
	}
	if descriptorBase64 == "" {
		t.Fatalf("proto descriptor %q not found in generated plugin", protoPath)
	}
	return descriptorBase64
}

func typeScriptRuntimeContractFixture() string {
	return `
import assert from "node:assert/strict";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { ErrorCode, McpError } from "@modelcontextprotocol/sdk/types.js";
import { registerRuntimeAPITools, type RuntimeAPIToolHandler } from "./runtime_contract_mcp.js";
import type { EchoRequest, EchoResponse } from "./runtime_contract_pb.js";

type TestToolResult = {
  content: { type: "text"; text: string }[];
  structuredContent?: Record<string, unknown>;
  isError?: boolean;
};

let handlerCalls = 0;
let mode: "ok" | "business-error" | "bad-output" = "ok";

const handler: RuntimeAPIToolHandler = {
  echo(_ctx, request: EchoRequest): EchoResponse {
    handlerCalls += 1;
    assert.equal(request.$typeName, "runtime.v1.EchoRequest");
    assert.ok(request.payload instanceof Uint8Array);
    if (mode === "business-error") {
      throw new Error("business failed");
    }
    if (mode === "bad-output") {
      return {
        $typeName: "runtime.v1.EchoResponse",
        label: 123 as unknown as string,
        count: request.count,
        payload: request.payload,
      };
    }
    return {
      $typeName: "runtime.v1.EchoResponse",
      label: request.label.toUpperCase(),
      count: request.count + 1,
      payload: request.payload,
    };
  },
};

const server = new Server(
  { name: "generated-typescript-runtime", version: "0.0.0" },
  { capabilities: { tools: {} } },
);
registerRuntimeAPITools(server, handler, "");

const client = new Client(
  { name: "generated-typescript-runtime-client", version: "0.0.0" },
  { capabilities: {} },
);
const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
await server.connect(serverTransport);
await client.connect(clientTransport);

const listed = await client.listTools();
assert.equal(listed.tools.length, 1);
assert.equal(listed.tools[0]?.name, "Echo");
assert.equal(listed.tools[0]?.inputSchema.type, "object");
assert.deepEqual(listed.tools[0]?.inputSchema.required, ["label", "count", "payload"]);
assert.equal(listed.tools[0]?.outputSchema?.type, "object");

const valid = await client.callTool({
  name: "Echo",
  arguments: { label: "alpha", count: 41, payload: "AQID" },
}) as TestToolResult;
assert.equal(handlerCalls, 1);
assert.equal(valid.isError, undefined);
assert.deepEqual(valid.structuredContent, { label: "ALPHA", count: 42, payload: "AQID" });
assert.equal(valid.content.length, 1);
assert.deepEqual(JSON.parse(valid.content[0]?.text ?? ""), valid.structuredContent);

await expectInvalidParams(
  () => client.callTool({ name: "Echo", arguments: { label: 7, count: 1, payload: "AQID" } }),
  /invalid arguments for tool 'Echo'/,
);
assert.equal(handlerCalls, 1);

await expectInvalidParams(
  () => client.callTool({ name: "Echo", arguments: { label: "alpha", count: 1, payload: "%" } }),
  /invalid arguments for tool 'Echo'/,
);
assert.equal(handlerCalls, 1);

mode = "business-error";
const toolError = await client.callTool({
  name: "Echo",
  arguments: { label: "alpha", count: 1, payload: "AQID" },
}) as TestToolResult;
assert.equal(handlerCalls, 2);
assert.equal(toolError.isError, true);
assert.match(toolError.content[0]?.text ?? "", /business failed/);

mode = "bad-output";
await expectRuntimeBug(
  () => client.callTool({ name: "Echo", arguments: { label: "alpha", count: 1, payload: "AQID" } }),
  /mcpruntime:/,
);
assert.equal(handlerCalls, 3);

await client.close();
await server.close();

async function expectInvalidParams(run: () => Promise<unknown>, message: RegExp): Promise<void> {
  try {
    await run();
  } catch (error) {
    if (!(error instanceof McpError)) {
      assert.fail("expected McpError, got " + String(error));
    }
    assert.equal(error.code, ErrorCode.InvalidParams);
    assert.match(error.message, message);
    return;
  }
  assert.fail("expected InvalidParams error");
}

async function expectRuntimeBug(run: () => Promise<unknown>, message: RegExp): Promise<void> {
  try {
    await run();
  } catch (error) {
    if (!(error instanceof Error)) {
      assert.fail("expected Error, got " + String(error));
    }
    assert.match(error.message, message);
    return;
  }
  assert.fail("expected runtime bug error");
}
`
}

func typeScriptRuntimeFixtureTSConfig() string {
	return `
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "verbatimModuleSyntax": true,
    "types": ["node"],
    "rootDir": ".",
    "outDir": "dist"
  },
  "include": [
    "runtime/v1/runtime_contract_mcp.ts",
    "runtime/v1/runtime_contract_pb.ts",
    "runtime/v1/runtime.ts"
  ]
}
`
}
