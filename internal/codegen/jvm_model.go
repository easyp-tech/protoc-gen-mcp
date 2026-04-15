package codegen

import (
	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
)

type JVMFileModel struct {
	Language                Language
	ProtoPath               string
	GeneratedFilenamePrefix string
	ProtoPackage            string
	Services                []JVMServiceModel
	Types                   JVMTypeGraph
}

type JVMServiceModel struct {
	ProtoFullName string
	ProtoName     string
	Namespace     string
	Description   string
	HandlerName   string
	RegisterName  string
	Icons         []*mcpoptionsv1.Icon
	Methods       []JVMMethodModel
}

type JVMMethodModel struct {
	ProtoFullName    string
	ProtoName        string
	ToolName         string
	Title            string
	Description      string
	Examples         []string
	Deprecated       bool
	MethodName       string
	SchemaConst      string
	Input            JVMTypeRef
	Output           JVMTypeRef
	InputSchemaJSON  string
	OutputSchemaJSON string
	Annotations      *mcpoptionsv1.ToolAnnotations
	Icons            []*mcpoptionsv1.Icon
	TaskSupport      mcpoptionsv1.TaskSupport
}

type JVMTypeGraph struct {
	CurrentFile JVMTypeOwner
	Imports     []JVMImport
	Types       []JVMType
}

type JVMImport struct {
	ProtoPath               string
	GeneratedFilenamePrefix string
	ProtoPackage            string
}

type JVMTypeOwner struct {
	ProtoPath               string
	IsCurrentFile           bool
	GeneratedFilenamePrefix string
	ProtoPackage            string
}

type JVMTypeKind string

const (
	JVMTypeKindMessage JVMTypeKind = "message"
	JVMTypeKindEnum    JVMTypeKind = "enum"
)

type JVMType struct {
	Kind          JVMTypeKind
	ProtoFullName string
	ProtoName     string
	PublicName    string
	Owner         JVMTypeOwner
	NestingPath   []string
	Fields        []JVMField
	Oneofs        []JVMOneof
	EnumValues    []JVMEnumValue
	WellKnownType JVMWellKnownType
}

type JVMEnumValue struct {
	ProtoName string
	Number    int32
	Hidden    bool
}

type JVMField struct {
	ProtoName          string
	JSONName           string
	Number             int
	Type               JVMTypeRef
	IsRepeated         bool
	IsMap              bool
	MapKeyScalar       JVMScalar
	MapValue           *JVMTypeRef
	HasPresence        bool
	IsSchemaRequired   bool
	OneofProtoName     string
	OneofWrapperName   string
	VariantWrapperName string
}

type JVMOneof struct {
	ProtoName   string
	WrapperName string
	Variants    []JVMOneofVariant
}

type JVMOneofVariant struct {
	ProtoName   string
	FieldNumber int
	WrapperName string
	Type        JVMTypeRef
	HasPresence bool
}

type JVMTypeRef struct {
	ProtoFullName string
	ProtoName     string
	PublicName    string
	Owner         JVMTypeOwner
	Scalar        JVMScalar
	WellKnownType JVMWellKnownType
	IsEnum        bool
	IsMessage     bool
}

type JVMScalar string

const (
	JVMScalarUnknown JVMScalar = ""
	JVMScalarBool    JVMScalar = "bool"
	JVMScalarBytes   JVMScalar = "bytes"
	JVMScalarString  JVMScalar = "string"
	JVMScalarInt32   JVMScalar = "int32"
	JVMScalarUInt32  JVMScalar = "uint32"
	JVMScalarInt64   JVMScalar = "int64"
	JVMScalarUInt64  JVMScalar = "uint64"
	JVMScalarFloat   JVMScalar = "float"
	JVMScalarDouble  JVMScalar = "double"
)

type JVMWellKnownType string

const (
	JVMWellKnownTypeNone        JVMWellKnownType = ""
	JVMWellKnownTypeAny         JVMWellKnownType = "Any"
	JVMWellKnownTypeEmpty       JVMWellKnownType = "Empty"
	JVMWellKnownTypeTimestamp   JVMWellKnownType = "Timestamp"
	JVMWellKnownTypeDuration    JVMWellKnownType = "Duration"
	JVMWellKnownTypeFieldMask   JVMWellKnownType = "FieldMask"
	JVMWellKnownTypeStruct      JVMWellKnownType = "Struct"
	JVMWellKnownTypeValue       JVMWellKnownType = "Value"
	JVMWellKnownTypeListValue   JVMWellKnownType = "ListValue"
	JVMWellKnownTypeBoolValue   JVMWellKnownType = "BoolValue"
	JVMWellKnownTypeStringValue JVMWellKnownType = "StringValue"
	JVMWellKnownTypeBytesValue  JVMWellKnownType = "BytesValue"
	JVMWellKnownTypeInt32Value  JVMWellKnownType = "Int32Value"
	JVMWellKnownTypeUInt32Value JVMWellKnownType = "UInt32Value"
	JVMWellKnownTypeInt64Value  JVMWellKnownType = "Int64Value"
	JVMWellKnownTypeUInt64Value JVMWellKnownType = "UInt64Value"
	JVMWellKnownTypeFloatValue  JVMWellKnownType = "FloatValue"
	JVMWellKnownTypeDoubleValue JVMWellKnownType = "DoubleValue"
)
