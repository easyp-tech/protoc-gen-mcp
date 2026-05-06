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

func TestTypeScriptRuntimeContract_AdvancedProtoJSON(t *testing.T) {
	spikeDir := requireTypeScriptSDKSpike(t)
	const protoPath = "runtime/v1/advanced-contract.proto"
	plugin := newTempProtogenPlugin(t, map[string]string{
		protoPath: strings.Join([]string{
			`syntax = "proto3";`,
			`package runtime.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/runtime;runtimev1";`,
			`import "google/protobuf/any.proto";`,
			`import "google/protobuf/duration.proto";`,
			`import "google/protobuf/field_mask.proto";`,
			`import "google/protobuf/struct.proto";`,
			`import "google/protobuf/timestamp.proto";`,
			`import "google/protobuf/wrappers.proto";`,
			`service RuntimeAPI {`,
			`  rpc Scalar(ScalarRequest) returns (ScalarResponse);`,
			`  rpc Advanced(AdvancedRequest) returns (AdvancedResponse);`,
			`}`,
			`message ScalarRequest {`,
			`  bool bool_flag = 1;`,
			`  string text_value = 2;`,
			`  bytes bytes_value = 3;`,
			`  int32 int32_value = 4;`,
			`  sint32 sint32_value = 5;`,
			`  sfixed32 sfixed32_value = 6;`,
			`  uint32 uint32_value = 7;`,
			`  fixed32 fixed32_value = 8;`,
			`  int64 int64_value = 9;`,
			`  sint64 sint64_value = 10;`,
			`  sfixed64 sfixed64_value = 11;`,
			`  uint64 uint64_value = 12;`,
			`  fixed64 fixed64_value = 13;`,
			`  float float_value = 14;`,
			`  double double_value = 15;`,
			`  Status status = 16;`,
			`  Detail details = 17;`,
			`  repeated int32 samples = 18;`,
			`  optional bool optional_bool_flag = 19;`,
			`  optional string optional_text_value = 20;`,
			`  optional bytes optional_bytes_value = 21;`,
			`  optional int32 optional_int32_value = 22;`,
			`  optional uint64 optional_uint64_value = 23;`,
			`  optional float optional_float_value = 24;`,
			`  optional double optional_double_value = 25;`,
			`  optional Status optional_status = 26;`,
			`}`,
			`message ScalarResponse {`,
			`  bool bool_flag = 1;`,
			`  string text_value = 2;`,
			`  bytes bytes_value = 3;`,
			`  int32 int32_value = 4;`,
			`  sint32 sint32_value = 5;`,
			`  sfixed32 sfixed32_value = 6;`,
			`  uint32 uint32_value = 7;`,
			`  fixed32 fixed32_value = 8;`,
			`  int64 int64_value = 9;`,
			`  sint64 sint64_value = 10;`,
			`  sfixed64 sfixed64_value = 11;`,
			`  uint64 uint64_value = 12;`,
			`  fixed64 fixed64_value = 13;`,
			`  float float_value = 14;`,
			`  double double_value = 15;`,
			`  Status status = 16;`,
			`  Detail details = 17;`,
			`  repeated int32 samples = 18;`,
			`  optional bool optional_bool_flag = 19;`,
			`  optional string optional_text_value = 20;`,
			`  optional bytes optional_bytes_value = 21;`,
			`  optional int32 optional_int32_value = 22;`,
			`  optional uint64 optional_uint64_value = 23;`,
			`  optional float optional_float_value = 24;`,
			`  optional double optional_double_value = 25;`,
			`  optional Status optional_status = 26;`,
			`}`,
			`message AdvancedRequest {`,
			`  map<string, string> labels = 1;`,
			`  map<int32, string> quantities = 2;`,
			`  map<bool, string> toggles = 3;`,
			`  map<uint64, string> limits = 4;`,
			`  optional google.protobuf.Timestamp observed_at = 5;`,
			`  optional google.protobuf.Duration ttl = 6;`,
			`  optional google.protobuf.Struct payload = 7;`,
			`  optional google.protobuf.ListValue items = 8;`,
			`  optional google.protobuf.Value dynamic = 9;`,
			`  optional google.protobuf.StringValue note = 10;`,
			`  optional google.protobuf.Int64Value total = 11;`,
			`  optional google.protobuf.BoolValue enabled = 12;`,
			`  optional google.protobuf.DoubleValue ratio = 13;`,
			`  optional google.protobuf.FieldMask mask = 14;`,
			`  optional google.protobuf.BytesValue blob = 15;`,
			`  optional google.protobuf.Int32Value small_total = 16;`,
			`  optional google.protobuf.UInt32Value uint_total = 17;`,
			`  optional google.protobuf.UInt64Value huge_total = 18;`,
			`  optional google.protobuf.FloatValue weight = 19;`,
			`  optional double raw_ratio = 20;`,
			`  optional RecursiveNode tree = 21;`,
			`  optional google.protobuf.Any detail_any = 22;`,
			`  optional google.protobuf.Any duration_any = 23;`,
			`  oneof selector {`,
			`    string city_alias = 24;`,
			`    int64 city_id = 25;`,
			`    Detail city_details = 26;`,
			`  }`,
			`}`,
			`message AdvancedResponse {`,
			`  map<string, string> labels = 1;`,
			`  map<int32, string> quantities = 2;`,
			`  map<bool, string> toggles = 3;`,
			`  map<uint64, string> limits = 4;`,
			`  optional google.protobuf.Timestamp observed_at = 5;`,
			`  optional google.protobuf.Duration ttl = 6;`,
			`  optional google.protobuf.Struct payload = 7;`,
			`  optional google.protobuf.ListValue items = 8;`,
			`  optional google.protobuf.Value dynamic = 9;`,
			`  optional google.protobuf.StringValue note = 10;`,
			`  optional google.protobuf.Int64Value total = 11;`,
			`  optional google.protobuf.BoolValue enabled = 12;`,
			`  optional google.protobuf.DoubleValue ratio = 13;`,
			`  optional google.protobuf.FieldMask mask = 14;`,
			`  optional google.protobuf.BytesValue blob = 15;`,
			`  optional google.protobuf.Int32Value small_total = 16;`,
			`  optional google.protobuf.UInt32Value uint_total = 17;`,
			`  optional google.protobuf.UInt64Value huge_total = 18;`,
			`  optional google.protobuf.FloatValue weight = 19;`,
			`  optional double raw_ratio = 20;`,
			`  optional RecursiveNode tree = 21;`,
			`  optional google.protobuf.Any detail_any = 22;`,
			`  optional google.protobuf.Any duration_any = 23;`,
			`  oneof selector {`,
			`    string city_alias = 24;`,
			`    int64 city_id = 25;`,
			`    Detail city_details = 26;`,
			`  }`,
			`}`,
			`message Detail { string label = 1; }`,
			`message RecursiveNode {`,
			`  string name = 1;`,
			`  optional RecursiveNode child = 2;`,
			`  repeated RecursiveNode children = 3;`,
			`}`,
			`enum Status {`,
			`  STATUS_NONE = 0;`,
			`  STATUS_OK = 1;`,
			`  STATUS_FAILED = 2;`,
			`}`,
			``,
		}, "\n"),
	}, protoPath)
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript: %v", err)
	}

	tempProject, err := os.MkdirTemp(spikeDir, ".tmp-typescript-advanced-runtime-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript advanced runtime project under sdk-spike: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempProject); err != nil {
			t.Errorf("remove temporary TypeScript advanced runtime project: %v", err)
		}
	})

	sourceDir := filepath.Join(tempProject, "runtime", "v1")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create advanced runtime fixture source dir: %v", err)
	}

	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "advanced_contract_mcp.ts"), generatedFileContent(t, plugin, "runtime/v1/advanced_contract_mcp.ts"))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "advanced_contract_pb.ts"), []byte(protobufESAdvancedRuntimeFixtureModule(t, plugin, protoPath)))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "runtime.ts"), []byte(typeScriptAdvancedRuntimeContractFixture()))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "tsconfig.json"), []byte(typeScriptAdvancedRuntimeFixtureTSConfig()))

	compile := exec.Command("npm", "exec", "--prefix", spikeDir, "--", "tsc", "-p", filepath.Join(tempProject, "tsconfig.json"))
	compile.Dir = spikeDir
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	compileOutput, err := compile.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript advanced runtime contract failed NodeNext compile via npm exec:\n%s", string(compileOutput))
	}

	run := exec.Command("node", filepath.Join(tempProject, "dist", "runtime", "v1", "runtime.js"))
	run.Dir = spikeDir
	run.Env = append(os.Environ(), "NO_COLOR=1")
	runOutput, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript advanced runtime contract failed under node:\n%s", string(runOutput))
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

func protobufESAdvancedRuntimeFixtureModule(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	descriptorBase64 := protobufESFileDescriptorBase64(t, plugin, protoPath)
	return fmt.Sprintf(`
import type { Message } from "@bufbuild/protobuf";
import { fileDesc, messageDesc, type GenFile, type GenMessage } from "@bufbuild/protobuf/codegenv2";
import {
  file_google_protobuf_any,
  file_google_protobuf_duration,
  file_google_protobuf_field_mask,
  file_google_protobuf_struct,
  file_google_protobuf_timestamp,
  file_google_protobuf_wrappers,
} from "@bufbuild/protobuf/wkt";

export const file_runtime_v1_advanced_contract: GenFile = fileDesc(%q, [
  file_google_protobuf_any,
  file_google_protobuf_duration,
  file_google_protobuf_field_mask,
  file_google_protobuf_struct,
  file_google_protobuf_timestamp,
  file_google_protobuf_wrappers,
]);

export type ScalarRequest = Message<"runtime.v1.ScalarRequest"> & Record<string, unknown>;
export type ScalarResponse = Message<"runtime.v1.ScalarResponse"> & Record<string, unknown>;
export type AdvancedRequest = Message<"runtime.v1.AdvancedRequest"> & Record<string, unknown>;
export type AdvancedResponse = Message<"runtime.v1.AdvancedResponse"> & Record<string, unknown>;

export const ScalarRequestSchema: GenMessage<ScalarRequest> = messageDesc(file_runtime_v1_advanced_contract, 0);
export const ScalarResponseSchema: GenMessage<ScalarResponse> = messageDesc(file_runtime_v1_advanced_contract, 1);
export const AdvancedRequestSchema: GenMessage<AdvancedRequest> = messageDesc(file_runtime_v1_advanced_contract, 2);
export const AdvancedResponseSchema: GenMessage<AdvancedResponse> = messageDesc(file_runtime_v1_advanced_contract, 3);
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

func typeScriptAdvancedRuntimeContractFixture() string {
	return `
import assert from "node:assert/strict";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { registerRuntimeAPITools, type RuntimeAPIToolHandler } from "./advanced_contract_mcp.js";
import type { AdvancedRequest, AdvancedResponse, ScalarRequest, ScalarResponse } from "./advanced_contract_pb.js";

type TestToolResult = {
  content: { type: "text"; text: string }[];
  structuredContent?: Record<string, unknown>;
  isError?: boolean;
};

const handler: RuntimeAPIToolHandler = {
  scalar(_ctx, request: ScalarRequest): ScalarResponse {
    return {
      ...(request as Record<string, unknown>),
      $typeName: "runtime.v1.ScalarResponse",
    } as ScalarResponse;
  },
  advanced(_ctx, request: AdvancedRequest): AdvancedResponse {
    return {
      ...(request as Record<string, unknown>),
      $typeName: "runtime.v1.AdvancedResponse",
    } as AdvancedResponse;
  },
};

const server = new Server(
  { name: "generated-typescript-advanced-runtime", version: "0.0.0" },
  { capabilities: { tools: {} } },
);
registerRuntimeAPITools(server, handler, "");

const client = new Client(
  { name: "generated-typescript-advanced-runtime-client", version: "0.0.0" },
  { capabilities: {} },
);
const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
await server.connect(serverTransport);
await client.connect(clientTransport);

const listed = await client.listTools();
assert.deepEqual(listed.tools.map((tool) => tool.name).sort(), ["Advanced", "Scalar"]);
for (const tool of listed.tools) {
  assert.equal(tool.inputSchema.type, "object");
  assert.equal(tool.outputSchema?.type, "object");
}

const scalarArgs = {
  boolFlag: true,
  textValue: "scalar-demo",
  bytesValue: "aGVsbG8=",
  int32Value: -123,
  sint32Value: -321,
  sfixed32Value: -456,
  uint32Value: 123,
  fixed32Value: 456,
  int64Value: "-4567890123",
  sint64Value: "-77",
  sfixed64Value: "-88",
  uint64Value: "4567890123",
  fixed64Value: "42",
  floatValue: 1.25,
  doubleValue: 2.5,
  status: "STATUS_OK",
  details: { label: "plain" },
  samples: [1, 2, 3],
  optionalBoolFlag: true,
  optionalTextValue: "optional-demo",
  optionalBytesValue: "d29ybGQ=",
  optionalInt32Value: 7,
  optionalUint64Value: "15",
  optionalFloatValue: 3.5,
  optionalDoubleValue: 4.5,
  optionalStatus: "STATUS_FAILED",
};
const scalar = await client.callTool({ name: "Scalar", arguments: scalarArgs }) as TestToolResult;
assertSuccessfulTextMatchesStructured("Scalar", scalar);
assert.deepEqual(scalar.structuredContent, scalarArgs);

const nullableScalar = await client.callTool({
  name: "Scalar",
  arguments: {
    ...scalarArgs,
    optionalBoolFlag: null,
    optionalTextValue: null,
    optionalBytesValue: null,
    optionalInt32Value: null,
    optionalUint64Value: null,
    optionalFloatValue: null,
    optionalDoubleValue: null,
    optionalStatus: null,
  },
}) as TestToolResult;
assertSuccessfulTextMatchesStructured("Scalar nullable", nullableScalar);
assert.equal(nullableScalar.structuredContent?.optionalTextValue, undefined);
assert.equal(nullableScalar.structuredContent?.optionalStatus, undefined);

const advancedArgs = {
  labels: { env: "prod", team: "core" },
  quantities: { "1": "one", "2": "two" },
  toggles: { true: "enabled", false: "disabled" },
  limits: { "18446744073709551615": "max" },
  observedAt: "2026-03-09T10:11:12Z",
  ttl: "3600s",
  payload: { kind: "demo", nested: { ok: true } },
  items: ["a", 2, false, { x: "y" }],
  dynamic: { city: "Paris", score: 7 },
  note: "hello",
  total: "42",
  enabled: true,
  ratio: "NaN",
  mask: "labels,observedAt",
  blob: "aGVsbG8=",
  smallTotal: 7,
  uintTotal: 11,
  hugeTotal: "99",
  weight: 1.5,
  rawRatio: "-Infinity",
  tree: {
    name: "root",
    child: { name: "leaf" },
    children: [{ name: "branch" }],
  },
  detailAny: {
    "@type": "type.googleapis.com/runtime.v1.Detail",
    label: "from-any",
  },
  durationAny: {
    "@type": "type.googleapis.com/google.protobuf.Duration",
    value: "3600s",
  },
  cityId: "9876543210",
};
const advanced = await client.callTool({ name: "Advanced", arguments: advancedArgs }) as TestToolResult;
assertSuccessfulTextMatchesStructured("Advanced", advanced);
assert.deepEqual(advanced.structuredContent, {
  ...advancedArgs,
  tree: {
    name: "root",
    child: { name: "leaf", children: [] },
    children: [{ name: "branch", children: [] }],
  },
});

const nullableAdvanced = await client.callTool({
  name: "Advanced",
  arguments: {
    labels: null,
    quantities: null,
    toggles: null,
    limits: null,
    observedAt: null,
    ttl: null,
    payload: null,
    items: null,
    dynamic: null,
    note: null,
    total: null,
    enabled: null,
    ratio: null,
    mask: null,
    blob: null,
    smallTotal: null,
    uintTotal: null,
    hugeTotal: null,
    weight: null,
    rawRatio: null,
    tree: null,
    detailAny: null,
    durationAny: null,
    cityAlias: null,
  },
}) as TestToolResult;
assertSuccessfulTextMatchesStructured("Advanced nullable", nullableAdvanced);
assert.deepEqual(nullableAdvanced.structuredContent?.labels, {});
assert.equal(nullableAdvanced.structuredContent?.observedAt, undefined);
assert.equal(nullableAdvanced.structuredContent?.detailAny, undefined);
assert.equal(nullableAdvanced.structuredContent?.cityAlias, undefined);

await client.close();
await server.close();

function assertSuccessfulTextMatchesStructured(toolName: string, result: TestToolResult): void {
  assert.equal(result.isError, undefined, toolName + " returned tool error");
  assert.equal(result.content.length, 1, toolName + " content item count");
  assert.deepEqual(JSON.parse(result.content[0]?.text ?? ""), result.structuredContent);
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

func typeScriptAdvancedRuntimeFixtureTSConfig() string {
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
    "runtime/v1/advanced_contract_mcp.ts",
    "runtime/v1/advanced_contract_pb.ts",
    "runtime/v1/runtime.ts"
  ]
}
`
}
