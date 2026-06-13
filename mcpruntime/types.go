package mcpruntime

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// JSON-RPC 2.0 error codes per specification.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface for JSONRPCError.
func (e *JSONRPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("jsonrpc error %d: %s (%v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// Implementation describes an MCP server or client.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability indicates the server supports tools.
type ToolsCapability struct{}

// ResourcesCapability indicates the server supports resources.
type ResourcesCapability struct{}

// PromptsCapability indicates the server supports prompts.
type PromptsCapability struct{}

// Tool describes a registered MCP tool.
type Tool struct {
	Name         string             `json:"name"`
	Title        string             `json:"title,omitempty"`
	Description  string             `json:"description,omitempty"`
	InputSchema  *jsonschema.Schema `json:"inputSchema,omitempty"`
	OutputSchema *jsonschema.Schema `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations   `json:"annotations,omitempty"`
	Icons        []Icon             `json:"icons,omitempty"`
}

// ToolAnnotations carries behavioral hints about a tool.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

// Icon represents a tool or resource icon.
type Icon struct {
	URL      string `json:"url"`
	MIMEType string `json:"mimeType,omitempty"`
}

// CallToolRequest represents tools/call params.
type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is the result of a tools/call invocation.
type CallToolResult struct {
	Content           []Content       `json:"content,omitempty"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// Content is the interface for MCP content types.
type Content interface {
	contentType() string
}

// TextContent holds text content.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t *TextContent) contentType() string { return "text" }

// MarshalJSON implements json.Marshaler for TextContent, ensuring "type" is always "text".
func (t *TextContent) MarshalJSON() ([]byte, error) {
	type alias TextContent
	return json.Marshal(&struct {
		*alias
		Type string `json:"type"`
	}{
		alias: (*alias)(t),
		Type:  "text",
	})
}

// Role is an MCP role string ("user" or "assistant").
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Annotations describes resource annotations.
type Annotations struct {
	Audience []Role   `json:"audience,omitempty"`
	Priority *float64 `json:"priority,omitempty"`
}

// Resource describes a static MCP resource.
type Resource struct {
	URI         string       `json:"uri"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	MIMEType    string       `json:"mimeType,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceTemplate describes a templated MCP resource.
type ResourceTemplate struct {
	URITemplate string       `json:"uriTemplate"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	MIMEType    string       `json:"mimeType,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceContents holds the content of a read resource.
type ResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ReadResourceRequest represents resources/read params.
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the result of resources/read.
type ReadResourceResult struct {
	Contents []*ResourceContents `json:"contents"`
}

// Prompt describes a registered MCP prompt.
type Prompt struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Icons       []Icon           `json:"icons,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage is a message in a prompt response.
type PromptMessage struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"`
}

// GetPromptRequest represents prompts/get params.
type GetPromptRequest struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the result of prompts/get.
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// InitializeRequest represents the initialize request params.
type InitializeRequest struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ClientInfo      *Implementation `json:"clientInfo,omitempty"`
}

// InitializeResult is the result of the initialize request.
type InitializeResult struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    *ServerCapabilities `json:"capabilities"`
	ServerInfo      *Implementation     `json:"serverInfo"`
}
