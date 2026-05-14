package codegen

import mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"

type TypeScriptFileModel struct {
	Language                Language
	ProtoPath               string
	GeneratedFilenamePrefix string
	ProtoPackage            string
	CurrentFile             TypeScriptTypeOwner
	Imports                 []TypeScriptImport
	RegistryRefs            []TypeScriptRegistryRef
	Services                []TypeScriptServiceModel
}

type TypeScriptServiceModel struct {
	ProtoFullName string
	ProtoName     string
	Namespace     string
	Description   string
	HandlerName   string
	RegisterName  string
	Icons         []*mcpoptionsv1.Icon
	Methods       []TypeScriptMethodModel
}

type TypeScriptMethodModel struct {
	ProtoFullName    string
	ProtoName        string
	ToolName         string
	Title            string
	Description      string
	Examples         []string
	Deprecated       bool
	MethodName       string
	SchemaConst      string
	Input            TypeScriptTypeRef
	Output           TypeScriptTypeRef
	InputSchemaJSON  string
	OutputSchemaJSON string
	Annotations      *mcpoptionsv1.ToolAnnotations
	Icons            []*mcpoptionsv1.Icon
	// TaskSupport mcpoptionsv1.TaskSupport mirrors the shared method contract.
	TaskSupport mcpoptionsv1.TaskSupport
}

type TypeScriptTypeRef struct {
	ProtoFullName string
	ProtoName     string
	TypeName      string
	SchemaName    string
	Owner         TypeScriptTypeOwner
	RegistryRef   TypeScriptRegistryRef
}

type TypeScriptImport struct {
	ProtoPath               string
	GeneratedFilenamePrefix string
	ModuleSpecifier         string
	TypeNames               []string
	SchemaNames             []string
	RegistryRefs            []string
}

type TypeScriptTypeOwner struct {
	ProtoPath               string
	IsCurrentFile           bool
	GeneratedFilenamePrefix string
	ModuleSpecifier         string
	RegistryRef             TypeScriptRegistryRef
}

type TypeScriptRegistryRef struct {
	ProtoPath string
	RefName   string
}
