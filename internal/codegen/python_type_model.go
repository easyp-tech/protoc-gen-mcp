package codegen

type PythonTypeGraph struct {
	Runtime     PythonRuntime
	CurrentFile PythonTypeOwner
	Imports     []PythonImport
	Types       []PythonType
}

type PythonModuleRef struct {
	ModulePath     string
	ModuleAlias    string
	ModuleBasename string
}

type PythonImport struct {
	ProtoPath      string
	PublicModule   PythonModuleRef
	ProtobufModule PythonModuleRef
}

type PythonTypeOwner struct {
	ProtoPath      string
	IsCurrentFile  bool
	PublicModule   PythonModuleRef
	ProtobufModule PythonModuleRef
}

type PythonTypeKind string

const (
	PythonTypeKindMessage PythonTypeKind = "message"
	PythonTypeKindEnum    PythonTypeKind = "enum"
)

type PythonType struct {
	Kind          PythonTypeKind
	ProtoFullName string
	ProtoName     string
	PublicName    string
	Owner         PythonTypeOwner
	NestingPath   []string
	Fields        []PythonField
	Oneofs        []PythonOneof
	EnumValues    []PythonEnumValue
	WellKnownType PythonWellKnownType
}

type PythonEnumValue struct {
	ProtoName string
	Number    int32
	Hidden    bool
}

type PythonField struct {
	ProtoName          string
	JSONName           string
	Number             int
	Type               PythonTypeRef
	IsRepeated         bool
	IsMap              bool
	MapKeyScalar       PythonScalar
	MapValue           *PythonTypeRef
	HasPresence        bool
	IsSchemaRequired   bool
	OneofProtoName     string
	OneofWrapperName   string
	VariantWrapperName string
}

type PythonOneof struct {
	ProtoName   string
	WrapperName string
	Variants    []PythonOneofVariant
}

type PythonOneofVariant struct {
	ProtoName   string
	FieldNumber int
	WrapperName string
	Type        PythonTypeRef
	HasPresence bool
}

type PythonTypeRef struct {
	ProtoFullName string
	ProtoName     string
	PublicName    string
	Owner         PythonTypeOwner
	Scalar        PythonScalar
	WellKnownType PythonWellKnownType
	IsEnum        bool
	IsMessage     bool
}

type PythonScalar string

const (
	PythonScalarUnknown PythonScalar = ""
	PythonScalarBool    PythonScalar = "bool"
	PythonScalarBytes   PythonScalar = "bytes"
	PythonScalarString  PythonScalar = "string"
	PythonScalarInt32   PythonScalar = "int32"
	PythonScalarUInt32  PythonScalar = "uint32"
	PythonScalarInt64   PythonScalar = "int64"
	PythonScalarUInt64  PythonScalar = "uint64"
	PythonScalarFloat   PythonScalar = "float"
	PythonScalarDouble  PythonScalar = "double"
)

type PythonWellKnownType string

const (
	PythonWellKnownTypeNone        PythonWellKnownType = ""
	PythonWellKnownTypeAny         PythonWellKnownType = "Any"
	PythonWellKnownTypeEmpty       PythonWellKnownType = "Empty"
	PythonWellKnownTypeTimestamp   PythonWellKnownType = "Timestamp"
	PythonWellKnownTypeDuration    PythonWellKnownType = "Duration"
	PythonWellKnownTypeFieldMask   PythonWellKnownType = "FieldMask"
	PythonWellKnownTypeStruct      PythonWellKnownType = "Struct"
	PythonWellKnownTypeValue       PythonWellKnownType = "Value"
	PythonWellKnownTypeListValue   PythonWellKnownType = "ListValue"
	PythonWellKnownTypeBoolValue   PythonWellKnownType = "BoolValue"
	PythonWellKnownTypeStringValue PythonWellKnownType = "StringValue"
	PythonWellKnownTypeBytesValue  PythonWellKnownType = "BytesValue"
	PythonWellKnownTypeInt32Value  PythonWellKnownType = "Int32Value"
	PythonWellKnownTypeUInt32Value PythonWellKnownType = "UInt32Value"
	PythonWellKnownTypeInt64Value  PythonWellKnownType = "Int64Value"
	PythonWellKnownTypeUInt64Value PythonWellKnownType = "UInt64Value"
	PythonWellKnownTypeFloatValue  PythonWellKnownType = "FloatValue"
	PythonWellKnownTypeDoubleValue PythonWellKnownType = "DoubleValue"
)
