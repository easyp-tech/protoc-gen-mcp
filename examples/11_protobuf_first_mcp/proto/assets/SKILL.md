# Protobuf-first MCP showcase skill

## When to use

Use this MCP resource to learn how the demonstration server exposes generated
tools, prompts, and resources. This is a plain Markdown file—not a JSON object
and not an automatically installed host-native Skill.

## Instructions

1. Call `showcase_get_overview` to inspect the current volatile report count.
2. Read `showcase://status` for the same information via a JSON resource.
3. Read `showcase://notes/intro` to demonstrate a URI-template text resource.
4. Read `ui://showcase/dashboard` to retrieve the MCP Apps HTML resource.
5. Publish a report only when the bearer token includes `showcase:write`.

All report records exist only in process memory. Restarting the server clears
them. Demo bearer tokens are local-development fixtures, not real OAuth tokens.
