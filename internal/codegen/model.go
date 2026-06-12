package codegen

import (
	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
)

// FileModel is the shared semantic IR entrypoint for language-specific renderers.
type FileModel struct {
	ProtoPath               string
	GeneratedFilenamePrefix string
	Options                 Options
	PythonTypes             *PythonTypeGraph
	Services                []ServiceModel
	Prompts                 []PromptModel
	Resources               []ResourceModel
}

type ServiceModel struct {
	ProtoFullName string
	ProtoName     string
	Namespace     string
	Description   string
	Icons         []*mcpoptionsv1.Icon
	Methods       []MethodModel
}

type MethodModel struct {
	ProtoFullName    string
	ProtoName        string
	Name             string
	Title            string
	Description      string
	Examples         []string
	Deprecated       bool
	Input            TypeRef
	Output           TypeRef
	InputSchemaJSON  string
	OutputSchemaJSON string
	Annotations      *mcpoptionsv1.ToolAnnotations
	Icons            []*mcpoptionsv1.Icon
	// TaskSupport mcpoptionsv1.TaskSupport preserves execution.task_support.
	TaskSupport mcpoptionsv1.TaskSupport
}

type TypeRef struct {
	ProtoFullName    string
	ProtoDisplayName string
}

// PromptModel represents a single MCP prompt derived from a proto message
// marked with (mcp.options.v1.prompt).
type PromptModel struct {
	ProtoFullName string
	ProtoName     string
	Name          string
	Title         string
	Description   string
	Icons         []*mcpoptionsv1.Icon
	Arguments     []PromptArgumentModel
	Input         TypeRef
}

// PromptArgumentModel represents one argument of an MCP prompt.
type PromptArgumentModel struct {
	ProtoName   string
	Name        string
	Description string
	Required    bool
}

// ResourceModel represents a single MCP resource derived from a proto message
// marked with (mcp.options.v1.resource).
type ResourceModel struct {
	ProtoFullName string
	ProtoName     string
	Name          string
	Description   string
	URI           string // Non-empty for static resources
	URITemplate   string // Non-empty for template resources
	MIMEType      string // Defaults to "application/json"
	IsTemplate    bool
	Params        []ResourceParamModel
	Annotations   *mcpoptionsv1.ResourceAnnotations
	Icons         []*mcpoptionsv1.Icon
	Output        TypeRef
}

// ResourceParamModel represents one parameter from a URI template.
type ResourceParamModel struct {
	Name string
}

