package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	showcasev1 "github.com/easyp-tech/protoc-gen-mcp/examples/11_protobuf_first_mcp/proto"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type showcase struct {
	mu      sync.Mutex
	reports int32
}

func (s *showcase) GetOverview(_ context.Context, req *showcasev1.GetOverviewRequest) (*showcasev1.GetOverviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &showcasev1.GetOverviewResponse{
		Topic:       req.GetTopic(),
		Summary:     "Reports exist only in memory; MCP Apps is declared in protobuf.",
		ReportCount: s.reports,
	}, nil
}

func (s *showcase) PublishReport(_ context.Context, req *showcasev1.PublishReportRequest) (*showcasev1.PublishReportResponse, error) {
	if req.GetTitle() == "" || req.GetBody() == "" {
		return nil, errors.New("report title and body are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports++
	return &showcasev1.PublishReportResponse{
		ReportId: fmt.Sprintf("report-%d", s.reports),
		Status:   "published",
	}, nil
}

func (s *showcase) GuidancePrompt(_ context.Context, req *showcasev1.GuidancePrompt) ([]*mcp.PromptMessage, error) {
	return []*mcp.PromptMessage{{
		Role:    mcp.Role("user"),
		Content: &mcp.TextContent{Text: "Show me the tools, prompts, JSON status, Markdown skill, and MCP Apps UI for: " + req.GetTopic()},
	}}, nil
}

func (s *showcase) ReadServerStatus(_ context.Context) (*showcasev1.ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &showcasev1.ServerStatus{Ready: true, ReportCount: s.reports}, nil
}

func (s *showcase) ListNoteTexts(_ context.Context) ([]mcp.Resource, error) {
	return []mcp.Resource{
		{Name: "intro", URI: "showcase://notes/intro", MIMEType: "text/plain"},
		{Name: "faq", URI: "showcase://notes/faq", MIMEType: "text/plain"},
	}, nil
}

func (s *showcase) ReadNoteText(_ context.Context, id string) (*showcasev1.NoteText, error) {
	switch id {
	case "intro":
		return &showcasev1.NoteText{Text: "Welcome! This is plain text, not a JSON wrapper."}, nil
	case "faq":
		return &showcasev1.NoteText{Text: "This demo uses volatile storage and loopback-only dev auth."}, nil
	default:
		return nil, fmt.Errorf("unknown note %q", id)
	}
}

func (s *showcase) ReadBinaryAttachment(_ context.Context) (*showcasev1.BinaryAttachment, error) {
	return &showcasev1.BinaryAttachment{Data: []byte{0x00, 0x01, 0xff, 0x10}}, nil
}

// newShowcaseServer uses the generated factory. The .proto file supplies
// registration, metadata, server version and MCP Apps capabilities.
func newShowcaseServer(ctx context.Context) (*mcp.Server, error) {
	impl := &showcase{}
	return showcasev1.NewFile_examples_11_protobuf_first_mcp_proto_showcase_protoMCPServer(
		ctx, showcasev1.File_examples_11_protobuf_first_mcp_proto_showcase_protoMCPHandlers{
			ShowcaseAPI: impl,
			Prompts:     impl,
			Resources:   impl,
		})
}

// Demo bearer tokens are intentionally insecure. They are only permitted
// when the operator explicitly opts in and binds HTTP to a loopback address.
func demoVerifier(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	var scopes []string
	switch token {
	case "demo-read":
		scopes = []string{"showcase:read"}
	case "demo-write":
		scopes = []string{"showcase:read", "showcase:write"}
	default:
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{
		UserID:     "local-demo-user",
		Scopes:     scopes,
		Expiration: time.Now().Add(time.Hour),
	}, nil
}

func newShowcaseHTTPHandler(server *mcp.Server, verifier auth.TokenVerifier) (http.Handler, error) {
	return showcasev1.NewFile_examples_11_protobuf_first_mcp_proto_showcase_protoMCPHTTPHandler(server, verifier)
}

func demoHTTPAddressIsLoopback(address string) bool {
	// net.SplitHostPort rejects missing or ambiguous host/port boundaries.
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
