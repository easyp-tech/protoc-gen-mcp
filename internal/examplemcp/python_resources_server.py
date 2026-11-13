"""Python counterpart of examplemcp.NewResourcesServer for the stdio resources contract."""

from __future__ import annotations

from pathlib import Path
import sys

import anyio
import mcp.server.lowlevel
import mcp.server.stdio
import mcp.types

_REPO_ROOT = Path(__file__).resolve().parents[2]
if str(_REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(_REPO_ROOT))

from internal.testproto.prompts.v1 import prompts_mcp
from internal.testproto.resources.v1 import resources_mcp

_SKILL_MARKDOWN = _REPO_ROOT / "internal/examplemcp/testdata/SKILL.md"
_REPORT_HTML = _REPO_ROOT / "internal/testproto/resources/v1/testdata/report.html"


class ResourcesHandler:
    def read_server_status(self) -> resources_mcp.ServerStatus:
        return resources_mcp.ServerStatus(healthy=True, uptime_seconds=123, version="v0.0.1")

    def list_user_profiles(self) -> list[mcp.types.Resource]:
        return [
            mcp.types.Resource(
                name="user_profile",
                uri="users://ada/profile",
                description="User profile information",
            )
        ]

    async def read_user_profile(self, user_id: str) -> resources_mcp.UserProfile:
        return resources_mcp.UserProfile(
            user_id=user_id,
            display_name="Ada Lovelace",
            email=f"{user_id}@example.com",
            active=True,
        )

    def list_documents(self) -> list[mcp.types.Resource]:
        return []

    def read_document(self, project_id: str, document_id: str) -> resources_mcp.Document:
        return resources_mcp.Document(
            title=f"doc {document_id} in {project_id}",
            body="body",
            author="Ada",
        )

    def read_skill_document(self) -> resources_mcp.SkillDocument:
        return resources_mcp.SkillDocument(markdown=_SKILL_MARKDOWN.read_text(encoding="utf-8"))

    def list_plain_texts(self) -> list[mcp.types.Resource]:
        return [mcp.types.Resource(name="plain_text", uri="docs://example/intro", mimeType="text/plain")]

    def read_plain_text(self, name: str) -> resources_mcp.PlainText:
        return resources_mcp.PlainText(body=f"document: {name}")

    def read_raw_binary(self) -> resources_mcp.RawBinary:
        return resources_mcp.RawBinary(data=bytes([0, 1, 2, 255]))

    def read_app_page(self) -> str:
        return _REPORT_HTML.read_text(encoding="utf-8")


def _user_message(text: str) -> mcp.types.GetPromptResult:
    return mcp.types.GetPromptResult(
        messages=[
            mcp.types.PromptMessage(
                role="user",
                content=mcp.types.TextContent(type="text", text=text),
            )
        ]
    )


class PromptsHandler:
    async def code_review(self, arguments: dict[str, str]) -> mcp.types.GetPromptResult:
        return _user_message(f"Review this {arguments['language']} code: {arguments['code']}")

    def summarize(self, arguments: dict[str, str]) -> mcp.types.GetPromptResult:
        return _user_message(f"Summarize: {arguments['content']}")

    def explain_error(self, arguments: dict[str, str]) -> mcp.types.GetPromptResult:
        return _user_message(f"Explain: {arguments['errorMessage']}")


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("protoc-gen-mcp-python-resources-server", version="v0.0.1")
    resources_mcp.register_resources_resources(server, ResourcesHandler())
    prompts_mcp.register_prompts_prompts(server, PromptsHandler())
    return server


async def run_stdio_server() -> None:
    server = new_server()
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(read_stream, write_stream, server.create_initialization_options())


def main() -> None:
    anyio.run(run_stdio_server)


if __name__ == "__main__":
    main()
