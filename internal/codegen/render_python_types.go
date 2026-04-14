package codegen

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

func renderPythonPublicTypes(generated *protogen.GeneratedFile, info pythonRenderInfo, model FileModel) error {
	graph := model.PythonTypes
	if graph == nil {
		return nil
	}

	generated.P("class _UnsetType:")
	generated.P("    __slots__ = ()")
	generated.P()
	generated.P("    def __repr__(self) -> str:")
	generated.P("        return \"UNSET\"")
	generated.P()
	generated.P("UNSET = _UnsetType()")
	generated.P()
	generated.P("JSONValue: TypeAlias = dict[str, Any] | list[Any] | str | int | float | bool | None")
	generated.P("ProtoAny: TypeAlias = dict[str, Any]")
	generated.P("Timestamp: TypeAlias = str")
	generated.P("Duration: TypeAlias = str")
	generated.P("FieldMask: TypeAlias = str")
	generated.P("Struct: TypeAlias = dict[str, Any]")
	generated.P("Value: TypeAlias = JSONValue")
	generated.P("ListValue: TypeAlias = list[Any]")
	generated.P("BoolValue: TypeAlias = bool")
	generated.P("StringValue: TypeAlias = str")
	generated.P("BytesValue: TypeAlias = bytes")
	generated.P("Int32Value: TypeAlias = int")
	generated.P("UInt32Value: TypeAlias = int")
	generated.P("Int64Value: TypeAlias = int")
	generated.P("UInt64Value: TypeAlias = int")
	generated.P("FloatValue: TypeAlias = float")
	generated.P("DoubleValue: TypeAlias = float")
	generated.P()
	generated.P("@dataclass(slots=True)")
	generated.P("class Empty:")
	generated.P("    pass")
	generated.P()

	for _, typ := range graph.Types {
		if !typ.Owner.IsCurrentFile {
			continue
		}
		switch typ.Kind {
		case PythonTypeKindEnum:
			renderPythonPublicEnum(generated, typ)
		case PythonTypeKindMessage:
			if err := renderPythonPublicMessage(generated, info, typ); err != nil {
				return err
			}
		}
	}

	return nil
}

func renderPythonPublicEnum(generated *protogen.GeneratedFile, typ PythonType) {
	generated.P("class ", typ.PublicName, "(enum.IntEnum):")
	rendered := 0
	for _, value := range typ.EnumValues {
		if value.Hidden {
			continue
		}
		generated.P("    ", value.ProtoName, " = ", value.Number)
		rendered++
	}
	if rendered == 0 {
		generated.P("    pass")
	}
	generated.P()
}

func renderPythonPublicMessage(generated *protogen.GeneratedFile, info pythonRenderInfo, typ PythonType) error {
	for _, oneof := range typ.Oneofs {
		generated.P("@dataclass(slots=True)")
		generated.P("class ", oneof.WrapperName, ":")
		generated.P("    pass")
		generated.P()
		for _, variant := range oneof.Variants {
			valueType := info.pythonPublicTypeRef(variant.Type)
			generated.P("@dataclass(slots=True)")
			generated.P("class ", variant.WrapperName, "(", oneof.WrapperName, "):")
			generated.P("    ", variant.ProtoName, ": ", valueType)
			generated.P()
		}
	}

	renderedFields, err := renderablePythonFields(info, typ)
	if err != nil {
		return err
	}

	generated.P("@dataclass(slots=True)")
	generated.P("class ", typ.PublicName, ":")
	if len(renderedFields) == 0 {
		generated.P("    pass")
		generated.P()
		return nil
	}
	for _, field := range renderedFields {
		switch {
		case field.defaultFactory != "":
			generated.P("    ", field.name, ": ", field.typ, " = field(default_factory=", field.defaultFactory, ")")
		case field.hasDefault:
			generated.P("    ", field.name, ": ", field.typ, " = ", field.defaultValue)
		default:
			generated.P("    ", field.name, ": ", field.typ)
		}
	}
	generated.P()
	return nil
}

type renderedPythonField struct {
	name           string
	typ            string
	hasDefault     bool
	defaultValue   string
	defaultFactory string
}

func renderablePythonFields(info pythonRenderInfo, typ PythonType) ([]renderedPythonField, error) {
	required := make([]renderedPythonField, 0, len(typ.Fields))
	optional := make([]renderedPythonField, 0, len(typ.Fields)+len(typ.Oneofs))

	for _, field := range typ.Fields {
		if field.OneofProtoName != "" {
			continue
		}

		fieldType, err := info.pythonFieldTypeAnnotation(field)
		if err != nil {
			return nil, err
		}
		rendered := renderedPythonField{
			name: field.ProtoName,
			typ:  fieldType,
		}
		switch {
		case field.IsRepeated:
			rendered.hasDefault = true
			rendered.defaultFactory = "list"
			optional = append(optional, rendered)
		case field.IsMap:
			rendered.hasDefault = true
			rendered.defaultFactory = "dict"
			optional = append(optional, rendered)
		case pythonFieldUsesUnset(field):
			rendered.typ += " | _UnsetType"
			rendered.hasDefault = true
			rendered.defaultValue = "UNSET"
			optional = append(optional, rendered)
		default:
			required = append(required, rendered)
		}
	}

	for _, oneof := range typ.Oneofs {
		optional = append(optional, renderedPythonField{
			name:         oneof.ProtoName,
			typ:          oneof.WrapperName + " | _UnsetType",
			hasDefault:   true,
			defaultValue: "UNSET",
		})
	}

	return append(required, optional...), nil
}

func pythonFieldUsesUnset(field PythonField) bool {
	if field.OneofProtoName != "" || field.IsRepeated || field.IsMap {
		return false
	}
	if field.IsSchemaRequired {
		return false
	}
	return field.HasPresence
}

func (p pythonRenderInfo) pythonFieldTypeAnnotation(field PythonField) (string, error) {
	switch {
	case field.IsRepeated:
		return "list[" + p.pythonPublicTypeRef(field.Type) + "]", nil
	case field.IsMap:
		if field.MapValue == nil {
			return "dict[str, Any]", nil
		}
		return "dict[" + pythonScalarAnnotation(field.MapKeyScalar) + ", " + p.pythonPublicTypeRef(*field.MapValue) + "]", nil
	default:
		return p.pythonPublicTypeRef(field.Type), nil
	}
}

func (p pythonRenderInfo) pythonPublicTypeRef(ref PythonTypeRef) string {
	if ref.IsEnum {
		return p.pythonPublicEnumTypeRef(ref)
	}
	switch {
	case ref.Scalar != PythonScalarUnknown:
		return pythonScalarAnnotation(ref.Scalar)
	case ref.WellKnownType != PythonWellKnownTypeNone:
		return pythonWellKnownTypeAnnotation(ref.WellKnownType)
	case ref.Owner.IsCurrentFile:
		return ref.PublicName
	default:
		return ref.Owner.PublicModule.ModuleAlias + "." + ref.PublicName
	}
}

func (p pythonRenderInfo) pythonPublicEnumTypeRef(ref PythonTypeRef) string {
	if ref.Owner.IsCurrentFile {
		return ref.PublicName
	}
	return ref.Owner.PublicModule.ModuleAlias + "." + ref.PublicName
}

func (t PythonType) typeRef() PythonTypeRef {
	return PythonTypeRef{
		ProtoFullName: t.ProtoFullName,
		ProtoName:     t.ProtoName,
		PublicName:    t.PublicName,
		Owner:         t.Owner,
		WellKnownType: t.WellKnownType,
		IsEnum:        t.Kind == PythonTypeKindEnum,
		IsMessage:     t.Kind == PythonTypeKindMessage,
	}
}

func pythonScalarAnnotation(scalar PythonScalar) string {
	switch scalar {
	case PythonScalarBool:
		return "bool"
	case PythonScalarBytes:
		return "bytes"
	case PythonScalarString:
		return "str"
	case PythonScalarInt32, PythonScalarUInt32, PythonScalarInt64, PythonScalarUInt64:
		return "int"
	case PythonScalarFloat, PythonScalarDouble:
		return "float"
	default:
		return "Any"
	}
}

func pythonWellKnownTypeAnnotation(kind PythonWellKnownType) string {
	switch kind {
	case PythonWellKnownTypeAny:
		return "ProtoAny"
	case PythonWellKnownTypeEmpty:
		return "Empty"
	case PythonWellKnownTypeTimestamp:
		return "Timestamp"
	case PythonWellKnownTypeDuration:
		return "Duration"
	case PythonWellKnownTypeFieldMask:
		return "FieldMask"
	case PythonWellKnownTypeStruct:
		return "Struct"
	case PythonWellKnownTypeValue:
		return "Value"
	case PythonWellKnownTypeListValue:
		return "ListValue"
	case PythonWellKnownTypeBoolValue:
		return "BoolValue"
	case PythonWellKnownTypeStringValue:
		return "StringValue"
	case PythonWellKnownTypeBytesValue:
		return "BytesValue"
	case PythonWellKnownTypeInt32Value:
		return "Int32Value"
	case PythonWellKnownTypeUInt32Value:
		return "UInt32Value"
	case PythonWellKnownTypeInt64Value:
		return "Int64Value"
	case PythonWellKnownTypeUInt64Value:
		return "UInt64Value"
	case PythonWellKnownTypeFloatValue:
		return "FloatValue"
	case PythonWellKnownTypeDoubleValue:
		return "DoubleValue"
	default:
		return "Any"
	}
}

func assertPythonTypesAvailable(model FileModel) error {
	if model.PythonTypes == nil {
		return fmt.Errorf("python type graph is required for python public type rendering")
	}
	return nil
}
