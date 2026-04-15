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
	TaskSupport      mcpoptionsv1.TaskSupport
}

type TypeRef struct {
	ProtoFullName    string
	ProtoDisplayName string
}
