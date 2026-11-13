package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
)

// TestPythonProtobufRuntime_ServesResources covers python_handler=protobuf,
// where resource handlers return pb2 messages instead of dataclasses. The
// dataclass mode runs over stdio in internal/examplemcp.
func TestPythonProtobufRuntime_ServesResources(t *testing.T) {
	plugin := newResourcesProtogenPlugin(t)
	file := plugin.FilesByPath["internal/testproto/resources/v1/resources.proto"]
	if file == nil {
		t.Fatal("resources proto file not found in plugin")
	}
	model, err := CollectFileModel(file, Options{
		Language:      LanguagePython,
		PythonRuntime: PythonRuntimeGoogleProtobuf,
		PythonHandler: PythonHandlerProtobuf,
	})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if err := renderPythonFile(plugin, model); err != nil {
		t.Fatalf("renderPythonFile: %v", err)
	}
	generated := generatedFileContent(t, plugin, "internal/testproto/resources/v1/resources_mcp.py")

	repoRoot := repoRootForPythonProtobufRuntimeTests(t)
	tempRoot := t.TempDir()
	packageDir := filepath.Join(tempRoot, "internal", "testproto", "resources", "v1")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, rel := range []string{"internal", "internal/testproto", "internal/testproto/resources", "internal/testproto/resources/v1"} {
		if err := os.WriteFile(filepath.Join(tempRoot, filepath.FromSlash(rel), "__init__.py"), nil, 0o644); err != nil {
			t.Fatalf("WriteFile __init__.py: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(packageDir, "resources_mcp.py"), generated, 0o644); err != nil {
		t.Fatalf("WriteFile resources_mcp.py: %v", err)
	}
	pb2, err := os.ReadFile(filepath.Join(repoRoot, "internal", "testproto", "resources", "v1", "resources_pb2.py"))
	if err != nil {
		t.Fatalf("ReadFile resources_pb2.py: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "resources_pb2.py"), pb2, 0o644); err != nil {
		t.Fatalf("WriteFile resources_pb2.py: %v", err)
	}

	cmd := pythontest.Command(t, "-c", `
import asyncio
import json
import mcp.server.lowlevel
import mcp.shared.exceptions
import mcp.types
from internal.testproto.resources.v1 import resources_mcp as module
from internal.testproto.resources.v1 import resources_pb2

class Handler:
    def read_server_status(self):
        return resources_pb2.ServerStatus(healthy=True, uptime_seconds=7)
    def list_user_profiles(self):
        return [mcp.types.Resource(name="user_profile", uri="users://ada/profile")]
    async def read_user_profile(self, user_id):
        return resources_pb2.UserProfile(user_id=user_id)
    def list_documents(self):
        return []
    def read_document(self, project_id, document_id):
        return resources_pb2.Document(title=project_id + "/" + document_id)
    def read_skill_document(self):
        return resources_pb2.SkillDocument(markdown="# Skill")
    def list_plain_texts(self):
        return []
    def read_plain_text(self, name):
        return resources_pb2.PlainText(body=name)
    def read_raw_binary(self):
        return resources_pb2.RawBinary(data=b"\x00\xff")
    def read_app_page(self):
        return b"<html></html>"

server = mcp.server.lowlevel.Server("protobuf-resources-test", version="1.0.0")
module.register_resources_resources(server, Handler(), namespace="pb")

async def read(uri):
    result = await server.request_handlers[mcp.types.ReadResourceRequest](
        mcp.types.ReadResourceRequest(params=mcp.types.ReadResourceRequestParams(uri=uri)),
    )
    return result.root.contents[0]

async def main():
    listed = await server.request_handlers[mcp.types.ListResourcesRequest](mcp.types.ListResourcesRequest())
    names = {str(resource.uri): resource.name for resource in listed.root.resources}
    assert names["server://status"] == "pb_server_status", names
    assert names["users://ada/profile"] == "user_profile", names

    status = await read("server://status")
    assert json.loads(status.text) == {"healthy": True, "uptimeSeconds": "7", "version": ""}, status.text

    profile = await read("users://a%2Fb/profile")
    assert json.loads(profile.text)["userId"] == "a/b", profile.text

    document = await read("projects://p+1/documents/d1")
    assert json.loads(document.text)["title"] == "p+1/d1", document.text

    blob = await read("blob://example/attachment")
    assert blob.blob == "AP8=", blob

    page = await read("ui://example/report")
    assert page.blob is not None and page.meta["ui"]["prefersBorder"] is True, page

    try:
        await read("users://ada%zz/profile")
    except mcp.shared.exceptions.McpError as error:
        assert error.error.code == mcp.types.INVALID_PARAMS, error.error
    else:
        raise AssertionError("malformed escape was accepted")

asyncio.run(main())
`)
	cmd.Dir = tempRoot
	cmd.Env = pythontest.Env(t, "PYTHONPATH="+tempRoot+string(os.PathListSeparator)+repoRoot)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("python protobuf resources runtime failed: %v\n%s", err, output)
	}
}
