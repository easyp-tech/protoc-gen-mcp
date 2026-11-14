package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/httpcontract"
)

// TestTypeScriptHTTPOAuthContract serves the config/v1 fixture through the
// generated createServerConfigMCPHttpHandler and checks it with the shared
// HTTP contract.
func TestTypeScriptHTTPOAuthContract(t *testing.T) {
	harness := requireTypeScriptSDKSpike(t)
	project := generateTypeScriptFixtureProject(t, harness, []string{"internal/testproto/config/v1/server_config.proto"})
	writeTypeScriptFixtureFile(t, filepath.Join(project, "server.ts"), []byte(`
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { create } from "@bufbuild/protobuf";
import { InvalidTokenError } from "@modelcontextprotocol/sdk/server/auth/errors.js";
import { createServerConfigMCPHttpHandler } from "./internal/testproto/config/v1/server_config_mcp.js";
import { CheckAccessResponseSchema } from "./internal/testproto/config/v1/server_config_pb.js";

const scopes: Record<string, string[]> = {
  reader: ["reports:read"],
  writer: ["reports:read", "reports:write"],
};

const handler = createServerConfigMCPHttpHandler(
  { secureAPI: { checkAccess: () => create(CheckAccessResponseSchema, { allowed: true }) } },
  {
    async verifyAccessToken(token: string) {
      const granted = scopes[token];
      if (granted === undefined) {
        throw new InvalidTokenError("invalid token");
      }
      return { token, clientId: "contract", scopes: granted, expiresAt: Math.floor(Date.now() / 1000) + 3600 };
    },
  },
);

const server = createServer(handler);
server.listen(0, "127.0.0.1", () => {
  const { port } = server.address() as AddressInfo;
  console.log("`+httpcontract.ListenPrefix+`http://127.0.0.1:" + port);
});
`))
	writeTypeScriptFixtureFile(t, filepath.Join(project, "tsconfig.json"), []byte(`{
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
  "include": ["**/*.ts"]
}
`))
	compile := exec.Command("npm", "exec", "--prefix", harness, "--", "tsc", "-p", filepath.Join(project, "tsconfig.json"))
	compile.Dir = harness
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("TypeScript HTTP server does not compile:\n%s", out)
	}
	cmd := exec.Command("node", filepath.Join(project, "dist", "server.js"))
	cmd.Dir = harness
	httpcontract.Run(t, cmd)
}
