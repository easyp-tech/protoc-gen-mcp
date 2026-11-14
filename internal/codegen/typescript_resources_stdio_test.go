package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/easyp-tech/protoc-gen-mcp/internal/stdiotest"
	"google.golang.org/protobuf/compiler/protogen"
)

// TestTypeScriptResourcesOverStdio serves the shared resources and prompts
// fixtures from generated TypeScript and checks them with the same stdio
// contract as the Go, Python and JVM servers.
func TestTypeScriptResourcesOverStdio(t *testing.T) {
	spikeDir := requireTypeScriptSDKSpike(t)
	root := repoRoot(t)

	const resourcesProto = "internal/testproto/resources/v1/resources.proto"
	const promptsProto = "internal/testproto/prompts/v1/prompts.proto"
	plugin := newCompiledProtogenPlugin(t, []string{resourcesProto, promptsProto}, &protocompile.SourceResolver{
		ImportPaths: []string{root},
	})
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript resources fixture: %v", err)
	}

	tempProject, err := os.MkdirTemp(spikeDir, ".tmp-typescript-resources-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript project under testdata/node: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempProject); err != nil {
			t.Errorf("remove temporary TypeScript project: %v", err)
		}
	})

	resourcesDir := filepath.Join(tempProject, "internal", "testproto", "resources", "v1")
	promptsDir := filepath.Join(tempProject, "internal", "testproto", "prompts", "v1")
	for _, dir := range []string{resourcesDir, promptsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	writeTypeScriptFixtureFile(t, filepath.Join(resourcesDir, "resources_mcp.ts"), generatedFileContent(t, plugin, "internal/testproto/resources/v1/resources_mcp.ts"))
	writeTypeScriptFixtureFile(t, filepath.Join(resourcesDir, "resources_pb.ts"), []byte(protobufESResourcesFixtureModule(t, plugin, resourcesProto)))
	writeTypeScriptFixtureFile(t, filepath.Join(promptsDir, "prompts_mcp.ts"), generatedFileContent(t, plugin, "internal/testproto/prompts/v1/prompts_mcp.ts"))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "server.ts"), []byte(typeScriptResourcesServerFixture(root)))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "tsconfig.json"), []byte(`{
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
  "include": ["server.ts", "internal/**/*.ts"]
}
`))

	compile := exec.Command("npm", "exec", "--prefix", spikeDir, "--", "tsc", "-p", filepath.Join(tempProject, "tsconfig.json"))
	compile.Dir = spikeDir
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated TypeScript resources server failed NodeNext compile:\n%s", output)
	}

	cmd := exec.Command("node", filepath.Join(tempProject, "dist", "server.js"))
	cmd.Dir = spikeDir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	stdiotest.RunResourcesPromptsContract(t, cmd, stdiotest.ContractOptions{})
}

// protobufESResourcesFixtureModule stands in for protoc-gen-es output for
// resources.proto. fileDesc drops the mcp/options import: the options only
// annotate the descriptor and are not needed at runtime.
func protobufESResourcesFixtureModule(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()
	return fmt.Sprintf(`
import type { Message } from "@bufbuild/protobuf";
import { fileDesc, messageDesc, type GenFile, type GenMessage } from "@bufbuild/protobuf/codegenv2";

export const file_internal_testproto_resources_v1_resources: GenFile = fileDesc(%q);

export type ServerStatus = Message<"internal.testproto.resources.v1.ServerStatus"> & {
  healthy: boolean;
  uptimeSeconds: bigint;
  version: string;
};
export type UserProfile = Message<"internal.testproto.resources.v1.UserProfile"> & {
  userId: string;
  displayName: string;
  email: string;
  active: boolean;
};
export type Document = Message<"internal.testproto.resources.v1.Document"> & {
  title: string;
  body: string;
  author: string;
};
export type SkillDocument = Message<"internal.testproto.resources.v1.SkillDocument"> & { markdown: string };
export type PlainText = Message<"internal.testproto.resources.v1.PlainText"> & { body: string };
export type RawBinary = Message<"internal.testproto.resources.v1.RawBinary"> & { data: Uint8Array };

export const ServerStatusSchema: GenMessage<ServerStatus> = messageDesc(file_internal_testproto_resources_v1_resources, 0);
export const UserProfileSchema: GenMessage<UserProfile> = messageDesc(file_internal_testproto_resources_v1_resources, 1);
export const DocumentSchema: GenMessage<Document> = messageDesc(file_internal_testproto_resources_v1_resources, 2);
export const SkillDocumentSchema: GenMessage<SkillDocument> = messageDesc(file_internal_testproto_resources_v1_resources, 3);
export const PlainTextSchema: GenMessage<PlainText> = messageDesc(file_internal_testproto_resources_v1_resources, 4);
export const RawBinarySchema: GenMessage<RawBinary> = messageDesc(file_internal_testproto_resources_v1_resources, 5);
`, protobufESFileDescriptorBase64(t, plugin, protoPath))
}

func typeScriptResourcesServerFixture(root string) string {
	return fmt.Sprintf(`
import { readFileSync } from "node:fs";
import { create } from "@bufbuild/protobuf";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import type { GetPromptResult } from "@modelcontextprotocol/sdk/types.js";
import { registerPromptsPrompts } from "./internal/testproto/prompts/v1/prompts_mcp.js";
import { createResourcesMCPServer } from "./internal/testproto/resources/v1/resources_mcp.js";
import {
  DocumentSchema,
  PlainTextSchema,
  RawBinarySchema,
  ServerStatusSchema,
  SkillDocumentSchema,
  UserProfileSchema,
} from "./internal/testproto/resources/v1/resources_pb.js";

const skillMarkdown = readFileSync(%q, "utf8");
const reportHtml = readFileSync(%q, "utf8");

function userMessage(text: string): GetPromptResult {
  return { messages: [{ role: "user", content: { type: "text", text } }] };
}

const server = createResourcesMCPServer({ resources: {
  readServerStatus: () => create(ServerStatusSchema, { healthy: true, uptimeSeconds: 123n, version: "v0.0.1" }),
  listUserProfiles: () => [{ name: "user_profile", uri: "users://ada/profile", description: "User profile information" }],
  readUserProfile: async (userId) =>
    create(UserProfileSchema, { userId, displayName: "Ada Lovelace", email: userId + "@example.com", active: true }),
  listDocuments: () => [],
  readDocument: (projectId, documentId) =>
    create(DocumentSchema, { title: "doc " + documentId + " in " + projectId, body: "body", author: "Ada" }),
  readSkillDocument: () => create(SkillDocumentSchema, { markdown: skillMarkdown }),
  listPlainTexts: () => [{ name: "plain_text", uri: "docs://example/intro", mimeType: "text/plain" }],
  readPlainText: (name) => create(PlainTextSchema, { body: "document: " + name }),
  readRawBinary: () => create(RawBinarySchema, { data: new Uint8Array([0, 1, 2, 255]) }),
  readAppPage: () => reportHtml,
} });

registerPromptsPrompts(server, {
  codeReview: async (args) => userMessage("Review this " + args.language + " code: " + args.code),
  summarize: (args) => userMessage("Summarize: " + args.content),
  explainError: (args) => userMessage("Explain: " + args.errorMessage),
});

await server.connect(new StdioServerTransport());
`, filepath.Join(root, "internal", "examplemcp", "testdata", "SKILL.md"), filepath.Join(root, "internal", "testproto", "resources", "v1", "testdata", "report.html"))
}
