// The client is a runnable MCP smoke client, not a browser UI host.
// It shows how a remote caller sees the proto-generated tool, prompt and
// resource contracts, including denied write calls for read-only tokens.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copied := req.Clone(req.Context())
	copied.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copied)
}

func main() {
	endpoint := flag.String("url", "http://127.0.0.1:8080/mcp", "MCP Streamable HTTP endpoint")
	token := flag.String("token", "demo-read", "local development bearer token")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "showcase-smoke-client", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: *endpoint,
		HTTPClient: &http.Client{Timeout: 12*time.Second, Transport: bearerTransport{
			token: *token, base: http.DefaultTransport,
		}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil { log.Fatalf("connect MCP: %v", err) }
	defer session.Close()

	init := session.InitializeResult()
	if init != nil {
		fmt.Printf("Connected to %s (protocol %s)\n", *endpoint, init.ProtocolVersion)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil { log.Fatal(err) }
	for _, tool := range tools.Tools {
		fmt.Printf("Tool: %s", tool.Name)
		if tool.Meta != nil { fmt.Printf(" _meta=%v", tool.Meta) }
		fmt.Println()
	}

	view, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "showcase_get_overview",
		Arguments: map[string]any{"topic": "MCP Apps"},
	})
	if err != nil { log.Fatal(err) }
	fmt.Printf("Overview: %s\n", contentText(view.Content))

	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil { log.Fatal(err) }
	for _, prompt := range prompts.Prompts { fmt.Printf("Prompt: %s\n", prompt.Name) }
	guide, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "showcase_guide", Arguments: map[string]string{"topic":"generated contracts"},
	})
	if err != nil { log.Fatal(err) }
	fmt.Printf("Guide prompt messages: %d\n", len(guide.Messages))

	resources, err := session.ListResources(ctx, nil)
	if err != nil { log.Fatal(err) }
	for _, resource := range resources.Resources { fmt.Printf("Resource: %s [%s]\n", resource.URI, resource.MIMEType) }

	templates, err := session.ListResourceTemplates(ctx, nil)
	if err != nil { log.Fatal(err) }
	for _, tmpl := range templates.ResourceTemplates { fmt.Printf("Resource template: %s\n", tmpl.URITemplate) }

	for _, uri := range []string{
		"showcase://status",
		"showcase://notes/intro",
		"showcase://SKILL.md",
		"ui://showcase/dashboard",
		"showcase://attachment",
	} {
		result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil { log.Fatalf("read %s: %v", uri, err) }
		if len(result.Contents) == 0 { log.Fatalf("read %s: no contents", uri) }
		content := result.Contents[0]
		if len(content.Blob) > 0 {
			fmt.Printf("Read %-30s [%s] %d binary bytes\n", uri, content.MIMEType, len(content.Blob))
		} else {
			text := strings.ReplaceAll(content.Text, "\n", " ")
			if len(text) > 100 { text = text[:100]+"…" }
			fmt.Printf("Read %-30s [%s] %s\n",uri, content.MIMEType, text)
		}
	}

	publish, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "showcase_publish_report",
		Arguments: map[string]any{"title":"From SDK client", "body":"Demo report"},
	})
	if err != nil { log.Fatal(err) }
	if publish.IsError {
		fmt.Printf("Publish denied (expected for demo-read): %s\n", contentText(publish.Content))
	} else {
		fmt.Printf("Publish succeeded (demo-write): %s\n", contentText(publish.Content))
	}
}

func contentText(items []mcp.Content) string {
	var parts []string
	for _, item := range items {
		if text, ok := item.(*mcp.TextContent); ok { parts = append(parts, text.Text) }
	}
	return strings.Join(parts, "\n")
}
