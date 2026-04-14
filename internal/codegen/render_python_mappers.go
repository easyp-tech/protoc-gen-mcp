package codegen

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

func renderPythonMappers(generated *protogen.GeneratedFile, info pythonRenderInfo, model FileModel) error {
	if model.PythonTypes == nil {
		return nil
	}
	if err := validatePythonMapperHelperNames(model.PythonTypes.Types); err != nil {
		return err
	}

	generated.P("def _enum_from_pb(enum_type: type[enum.IntEnum], value: int) -> enum.IntEnum:")
	generated.P("    return enum_type(value)")
	generated.P()

	generated.P("def _json_to_message(value: Any, message: Any) -> Any:")
	generated.P("    json_format.Parse(json.dumps(value), message)")
	generated.P("    return message")
	generated.P()

	renderPythonWellKnownTypeMappers(generated)

	for _, typ := range model.PythonTypes.Types {
		if typ.Kind != PythonTypeKindMessage {
			continue
		}
		if err := renderPythonMessageFromPBMapper(generated, info, typ); err != nil {
			return err
		}
		if err := renderPythonMessageToPBMapper(generated, info, typ); err != nil {
			return err
		}
	}

	return nil
}

func renderPythonWellKnownTypeMappers(generated *protogen.GeneratedFile) {
	generated.P("def _from_pb_any(message: any_pb2.Any) -> ProtoAny:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_any(value: ProtoAny) -> any_pb2.Any:")
	generated.P("    return _json_to_message(value, any_pb2.Any())")
	generated.P()
	generated.P("def _from_pb_timestamp(message: timestamp_pb2.Timestamp) -> Timestamp:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_timestamp(value: Timestamp) -> timestamp_pb2.Timestamp:")
	generated.P("    return _json_to_message(value, timestamp_pb2.Timestamp())")
	generated.P()
	generated.P("def _from_pb_duration(message: duration_pb2.Duration) -> Duration:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_duration(value: Duration) -> duration_pb2.Duration:")
	generated.P("    return _json_to_message(value, duration_pb2.Duration())")
	generated.P()
	generated.P("def _from_pb_field_mask(message: field_mask_pb2.FieldMask) -> FieldMask:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_field_mask(value: FieldMask) -> field_mask_pb2.FieldMask:")
	generated.P("    return _json_to_message(value, field_mask_pb2.FieldMask())")
	generated.P()
	generated.P("def _from_pb_struct(message: struct_pb2.Struct) -> Struct:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_struct(value: Struct) -> struct_pb2.Struct:")
	generated.P("    return _json_to_message(value, struct_pb2.Struct())")
	generated.P()
	generated.P("def _from_pb_list_value(message: struct_pb2.ListValue) -> ListValue:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_list_value(value: ListValue) -> struct_pb2.ListValue:")
	generated.P("    return _json_to_message(value, struct_pb2.ListValue())")
	generated.P()
	generated.P("def _from_pb_value(message: struct_pb2.Value) -> Value:")
	generated.P("    return json.loads(_message_to_json(message))")
	generated.P()
	generated.P("def _to_pb_value(value: Value) -> struct_pb2.Value:")
	generated.P("    return _json_to_message(value, struct_pb2.Value())")
	generated.P()
	generated.P("def _from_pb_empty(message: empty_pb2.Empty) -> Empty:")
	generated.P("    return Empty()")
	generated.P()
	generated.P("def _to_pb_empty(value: Empty) -> empty_pb2.Empty:")
	generated.P("    return empty_pb2.Empty()")
	generated.P()
	generated.P("def _from_pb_bool_value(message: wrappers_pb2.BoolValue) -> BoolValue:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_bool_value(value: BoolValue) -> wrappers_pb2.BoolValue:")
	generated.P("    return wrappers_pb2.BoolValue(value=value)")
	generated.P()
	generated.P("def _from_pb_string_value(message: wrappers_pb2.StringValue) -> StringValue:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_string_value(value: StringValue) -> wrappers_pb2.StringValue:")
	generated.P("    return wrappers_pb2.StringValue(value=value)")
	generated.P()
	generated.P("def _from_pb_bytes_value(message: wrappers_pb2.BytesValue) -> BytesValue:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_bytes_value(value: BytesValue) -> wrappers_pb2.BytesValue:")
	generated.P("    return wrappers_pb2.BytesValue(value=value)")
	generated.P()
	generated.P("def _from_pb_int32_value(message: wrappers_pb2.Int32Value) -> Int32Value:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_int32_value(value: Int32Value) -> wrappers_pb2.Int32Value:")
	generated.P("    return wrappers_pb2.Int32Value(value=value)")
	generated.P()
	generated.P("def _from_pb_uint32_value(message: wrappers_pb2.UInt32Value) -> UInt32Value:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_uint32_value(value: UInt32Value) -> wrappers_pb2.UInt32Value:")
	generated.P("    return wrappers_pb2.UInt32Value(value=value)")
	generated.P()
	generated.P("def _from_pb_int64_value(message: wrappers_pb2.Int64Value) -> Int64Value:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_int64_value(value: Int64Value) -> wrappers_pb2.Int64Value:")
	generated.P("    return wrappers_pb2.Int64Value(value=value)")
	generated.P()
	generated.P("def _from_pb_uint64_value(message: wrappers_pb2.UInt64Value) -> UInt64Value:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_uint64_value(value: UInt64Value) -> wrappers_pb2.UInt64Value:")
	generated.P("    return wrappers_pb2.UInt64Value(value=value)")
	generated.P()
	generated.P("def _from_pb_float_value(message: wrappers_pb2.FloatValue) -> FloatValue:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_float_value(value: FloatValue) -> wrappers_pb2.FloatValue:")
	generated.P("    return wrappers_pb2.FloatValue(value=value)")
	generated.P()
	generated.P("def _from_pb_double_value(message: wrappers_pb2.DoubleValue) -> DoubleValue:")
	generated.P("    return message.value")
	generated.P()
	generated.P("def _to_pb_double_value(value: DoubleValue) -> wrappers_pb2.DoubleValue:")
	generated.P("    return wrappers_pb2.DoubleValue(value=value)")
	generated.P()
}

func renderPythonMessageFromPBMapper(generated *protogen.GeneratedFile, info pythonRenderInfo, typ PythonType) error {
	pbType, err := info.pythonProtobufTypeRefForPublicRef(typ.typeRef())
	if err != nil {
		return err
	}
	dcType := info.pythonPublicTypeRef(typ.typeRef())
	generated.P("def ", pythonMapperHelperName("from_pb", typ), "(message: ", pbType, ") -> ", dcType, ":")

	for _, oneof := range typ.Oneofs {
		generated.P("    ", oneof.ProtoName, " = UNSET")
		generated.P("    ", oneof.ProtoName, "_case = message.WhichOneof(", quote(oneof.ProtoName), ")")
		for idx, variant := range oneof.Variants {
			keyword := "if"
			if idx > 0 {
				keyword = "elif"
			}
			valueExpr, err := info.pythonFromPBExpr(variant.Type, "message."+variant.ProtoName)
			if err != nil {
				return err
			}
			generated.P("    ", keyword, " ", oneof.ProtoName, "_case == ", quote(variant.ProtoName), ":")
			generated.P("        ", oneof.ProtoName, " = ", variant.WrapperName, "(", variant.ProtoName, "=", valueExpr, ")")
		}
	}

	generated.P("    return ", dcType, "(")
	for _, field := range typ.Fields {
		if field.OneofProtoName != "" {
			continue
		}
		valueExpr, err := info.pythonFromPBFieldExpr(field)
		if err != nil {
			return err
		}
		generated.P("        ", field.ProtoName, "=", valueExpr, ",")
	}
	for _, oneof := range typ.Oneofs {
		generated.P("        ", oneof.ProtoName, "=", oneof.ProtoName, ",")
	}
	generated.P("    )")
	generated.P()
	return nil
}

func renderPythonMessageToPBMapper(generated *protogen.GeneratedFile, info pythonRenderInfo, typ PythonType) error {
	pbType, err := info.pythonProtobufTypeRefForPublicRef(typ.typeRef())
	if err != nil {
		return err
	}
	dcType := info.pythonPublicTypeRef(typ.typeRef())
	generated.P("def ", pythonMapperHelperName("to_pb", typ), "(value: ", dcType, ") -> ", pbType, ":")
	generated.P("    message = ", pbType, "()")
	for _, field := range typ.Fields {
		if field.OneofProtoName != "" {
			continue
		}
		if err := renderPythonToPBFieldAssignment(generated, info, field, "value."+field.ProtoName, "message."+field.ProtoName, 1); err != nil {
			return err
		}
	}
	for _, oneof := range typ.Oneofs {
		generated.P("    if value.", oneof.ProtoName, " is not UNSET:")
		for idx, variant := range oneof.Variants {
			keyword := "if"
			if idx > 0 {
				keyword = "elif"
			}
			generated.P("        ", keyword, " isinstance(value.", oneof.ProtoName, ", ", variant.WrapperName, "):")
			if err := renderPythonToPBSingularAssignment(generated, info, variant.Type, "value."+oneof.ProtoName+"."+variant.ProtoName, "message."+variant.ProtoName, 3); err != nil {
				return err
			}
		}
		generated.P("        else:")
		generated.P("            raise TypeError(\"unsupported ", oneof.WrapperName, " variant: \" + type(value.", oneof.ProtoName, ").__name__)")
		generated.P()
	}
	generated.P("    return message")
	generated.P()
	return nil
}

func renderPythonToPBFieldAssignment(generated *protogen.GeneratedFile, info pythonRenderInfo, field PythonField, sourceExpr, targetExpr string, indent int) error {
	switch {
	case field.IsRepeated:
		return renderPythonToPBRepeatedAssignment(generated, info, field.Type, sourceExpr, targetExpr, indent)
	case field.IsMap:
		if field.MapValue == nil {
			return nil
		}
		return renderPythonToPBMapAssignment(generated, info, field, sourceExpr, targetExpr, indent)
	case pythonFieldUsesUnset(field):
		generated.P(pythonIndent(indent), "if ", sourceExpr, " is not UNSET:")
		return renderPythonToPBSingularAssignment(generated, info, field.Type, sourceExpr, targetExpr, indent+1)
	default:
		return renderPythonToPBSingularAssignment(generated, info, field.Type, sourceExpr, targetExpr, indent)
	}
}

func renderPythonToPBRepeatedAssignment(generated *protogen.GeneratedFile, info pythonRenderInfo, ref PythonTypeRef, sourceExpr, targetExpr string, indent int) error {
	prefix := pythonIndent(indent)
	switch {
	case ref.Scalar != PythonScalarUnknown:
		generated.P(prefix, targetExpr, ".extend(", sourceExpr, ")")
	case ref.IsEnum:
		generated.P(prefix, targetExpr, ".extend(int(item) for item in ", sourceExpr, ")")
	default:
		itemExpr, err := info.pythonToPBExpr(ref, "item")
		if err != nil {
			return err
		}
		generated.P(prefix, targetExpr, ".extend(", itemExpr, " for item in ", sourceExpr, ")")
	}
	return nil
}

func renderPythonToPBMapAssignment(generated *protogen.GeneratedFile, info pythonRenderInfo, field PythonField, sourceExpr, targetExpr string, indent int) error {
	prefix := pythonIndent(indent)
	valueRef := *field.MapValue
	switch {
	case valueRef.Scalar != PythonScalarUnknown:
		generated.P(prefix, targetExpr, ".update(", sourceExpr, ")")
	case valueRef.IsEnum:
		generated.P(prefix, targetExpr, ".update({key: int(item) for key, item in ", sourceExpr, ".items()})")
	default:
		itemExpr, err := info.pythonToPBExpr(valueRef, "item")
		if err != nil {
			return err
		}
		generated.P(prefix, "for key, item in ", sourceExpr, ".items():")
		generated.P(prefix, "    ", targetExpr, "[key].CopyFrom(", itemExpr, ")")
	}
	return nil
}

func renderPythonToPBSingularAssignment(generated *protogen.GeneratedFile, info pythonRenderInfo, ref PythonTypeRef, sourceExpr, targetExpr string, indent int) error {
	prefix := pythonIndent(indent)
	valueExpr, err := info.pythonToPBExpr(ref, sourceExpr)
	if err != nil {
		return err
	}
	switch {
	case ref.Scalar != PythonScalarUnknown || ref.IsEnum:
		generated.P(prefix, targetExpr, " = ", valueExpr)
	default:
		generated.P(prefix, targetExpr, ".CopyFrom(", valueExpr, ")")
	}
	return nil
}

func (p pythonRenderInfo) pythonFromPBFieldExpr(field PythonField) (string, error) {
	switch {
	case field.IsRepeated:
		if field.Type.Scalar != PythonScalarUnknown {
			return "list(message." + field.ProtoName + ")", nil
		}
		itemExpr, err := p.pythonFromPBExpr(field.Type, "item")
		if err != nil {
			return "", err
		}
		return "[" + itemExpr + " for item in message." + field.ProtoName + "]", nil
	case field.IsMap:
		if field.MapValue == nil {
			return "dict(message." + field.ProtoName + ")", nil
		}
		if field.MapValue.Scalar != PythonScalarUnknown {
			return "dict(message." + field.ProtoName + ")", nil
		}
		valueExpr, err := p.pythonFromPBExpr(*field.MapValue, "item")
		if err != nil {
			return "", err
		}
		return "{key: " + valueExpr + " for key, item in message." + field.ProtoName + ".items()}", nil
	default:
		baseExpr, err := p.pythonFromPBExpr(field.Type, "message."+field.ProtoName)
		if err != nil {
			return "", err
		}
		if pythonFieldUsesUnset(field) {
			return baseExpr + " if message.HasField(" + quote(field.ProtoName) + ") else UNSET", nil
		}
		return baseExpr, nil
	}
}

func (p pythonRenderInfo) pythonFromPBExpr(ref PythonTypeRef, expr string) (string, error) {
	switch {
	case ref.Scalar != PythonScalarUnknown:
		return expr, nil
	case ref.IsEnum:
		return "_enum_from_pb(" + p.pythonPublicEnumTypeRef(ref) + ", " + expr + ")", nil
	case ref.WellKnownType != PythonWellKnownTypeNone:
		return pythonWellKnownMapperFunc("from_pb", ref.WellKnownType) + "(" + expr + ")", nil
	case ref.IsMessage:
		fn, err := p.pythonMapperHelperForRef("from_pb", ref)
		if err != nil {
			return "", err
		}
		return fn + "(" + expr + ")", nil
	default:
		return expr, nil
	}
}

func (p pythonRenderInfo) pythonToPBExpr(ref PythonTypeRef, expr string) (string, error) {
	switch {
	case ref.Scalar != PythonScalarUnknown:
		return expr, nil
	case ref.IsEnum:
		return "int(" + expr + ")", nil
	case ref.WellKnownType != PythonWellKnownTypeNone:
		return pythonWellKnownMapperFunc("to_pb", ref.WellKnownType) + "(" + expr + ")", nil
	case ref.IsMessage:
		fn, err := p.pythonMapperHelperForRef("to_pb", ref)
		if err != nil {
			return "", err
		}
		return fn + "(" + expr + ")", nil
	default:
		return expr, nil
	}
}

func (p pythonRenderInfo) pythonMapperHelperForRef(prefix string, ref PythonTypeRef) (string, error) {
	if ref.WellKnownType != PythonWellKnownTypeNone {
		return pythonWellKnownMapperFunc(prefix, ref.WellKnownType), nil
	}
	typ, ok := p.publicTypes[ref.ProtoFullName]
	if !ok {
		return "", fmt.Errorf("public python type %q not found during mapper render", ref.ProtoFullName)
	}
	return pythonMapperHelperName(prefix, typ), nil
}

func (p pythonRenderInfo) pythonProtobufTypeRefForPublicRef(ref PythonTypeRef) (string, error) {
	if ref.WellKnownType != PythonWellKnownTypeNone {
		return pythonWellKnownProtobufType(ref.WellKnownType), nil
	}
	return p.pythonProtobufTypeRef(TypeRef{ProtoFullName: ref.ProtoFullName})
}

func pythonMapperHelperName(prefix string, typ PythonType) string {
	base := toSnakeCase(typ.PublicName)
	if typ.Owner.IsCurrentFile {
		return "_" + prefix + "_" + base
	}
	return "_" + prefix + "_" + toSnakeCase(typ.Owner.PublicModule.ModuleAlias) + "_" + base
}

func validatePythonMapperHelperNames(types []PythonType) error {
	owners := map[string]string{}
	for _, name := range pythonWellKnownHelperNames() {
		owners[name] = "built-in well-known type helper"
	}
	for _, typ := range types {
		if typ.Kind != PythonTypeKindMessage {
			continue
		}
		for _, prefix := range []string{"from_pb", "to_pb"} {
			helper := pythonMapperHelperName(prefix, typ)
			owner := fmt.Sprintf("%s mapper for %s", prefix, typ.ProtoFullName)
			if existing, exists := owners[helper]; exists {
				return fmt.Errorf("python mapper helper name collision for %q between %s and %s", helper, existing, owner)
			}
			owners[helper] = owner
		}
	}
	return nil
}

func pythonWellKnownHelperNames() []string {
	names := make([]string, 0, 2*len(pythonSupportedWellKnownTypes()))
	for _, kind := range pythonSupportedWellKnownTypes() {
		names = append(names, pythonWellKnownMapperFunc("from_pb", kind))
		names = append(names, pythonWellKnownMapperFunc("to_pb", kind))
	}
	return names
}

func pythonSupportedWellKnownTypes() []PythonWellKnownType {
	return []PythonWellKnownType{
		PythonWellKnownTypeAny,
		PythonWellKnownTypeEmpty,
		PythonWellKnownTypeTimestamp,
		PythonWellKnownTypeDuration,
		PythonWellKnownTypeFieldMask,
		PythonWellKnownTypeStruct,
		PythonWellKnownTypeValue,
		PythonWellKnownTypeListValue,
		PythonWellKnownTypeBoolValue,
		PythonWellKnownTypeStringValue,
		PythonWellKnownTypeBytesValue,
		PythonWellKnownTypeInt32Value,
		PythonWellKnownTypeUInt32Value,
		PythonWellKnownTypeInt64Value,
		PythonWellKnownTypeUInt64Value,
		PythonWellKnownTypeFloatValue,
		PythonWellKnownTypeDoubleValue,
	}
}

func pythonWellKnownMapperFunc(prefix string, kind PythonWellKnownType) string {
	var suffix string
	switch kind {
	case PythonWellKnownTypeAny:
		suffix = "any"
	case PythonWellKnownTypeEmpty:
		suffix = "empty"
	case PythonWellKnownTypeTimestamp:
		suffix = "timestamp"
	case PythonWellKnownTypeDuration:
		suffix = "duration"
	case PythonWellKnownTypeFieldMask:
		suffix = "field_mask"
	case PythonWellKnownTypeStruct:
		suffix = "struct"
	case PythonWellKnownTypeValue:
		suffix = "value"
	case PythonWellKnownTypeListValue:
		suffix = "list_value"
	case PythonWellKnownTypeBoolValue:
		suffix = "bool_value"
	case PythonWellKnownTypeStringValue:
		suffix = "string_value"
	case PythonWellKnownTypeBytesValue:
		suffix = "bytes_value"
	case PythonWellKnownTypeInt32Value:
		suffix = "int32_value"
	case PythonWellKnownTypeUInt32Value:
		suffix = "uint32_value"
	case PythonWellKnownTypeInt64Value:
		suffix = "int64_value"
	case PythonWellKnownTypeUInt64Value:
		suffix = "uint64_value"
	case PythonWellKnownTypeFloatValue:
		suffix = "float_value"
	case PythonWellKnownTypeDoubleValue:
		suffix = "double_value"
	default:
		suffix = toSnakeCase(string(kind))
	}
	return "_" + prefix + "_" + suffix
}

func pythonWellKnownProtobufType(kind PythonWellKnownType) string {
	switch kind {
	case PythonWellKnownTypeAny:
		return "any_pb2.Any"
	case PythonWellKnownTypeEmpty:
		return "empty_pb2.Empty"
	case PythonWellKnownTypeTimestamp:
		return "timestamp_pb2.Timestamp"
	case PythonWellKnownTypeDuration:
		return "duration_pb2.Duration"
	case PythonWellKnownTypeFieldMask:
		return "field_mask_pb2.FieldMask"
	case PythonWellKnownTypeStruct:
		return "struct_pb2.Struct"
	case PythonWellKnownTypeValue:
		return "struct_pb2.Value"
	case PythonWellKnownTypeListValue:
		return "struct_pb2.ListValue"
	case PythonWellKnownTypeBoolValue:
		return "wrappers_pb2.BoolValue"
	case PythonWellKnownTypeStringValue:
		return "wrappers_pb2.StringValue"
	case PythonWellKnownTypeBytesValue:
		return "wrappers_pb2.BytesValue"
	case PythonWellKnownTypeInt32Value:
		return "wrappers_pb2.Int32Value"
	case PythonWellKnownTypeUInt32Value:
		return "wrappers_pb2.UInt32Value"
	case PythonWellKnownTypeInt64Value:
		return "wrappers_pb2.Int64Value"
	case PythonWellKnownTypeUInt64Value:
		return "wrappers_pb2.UInt64Value"
	case PythonWellKnownTypeFloatValue:
		return "wrappers_pb2.FloatValue"
	case PythonWellKnownTypeDoubleValue:
		return "wrappers_pb2.DoubleValue"
	default:
		return "Any"
	}
}

func pythonIndent(level int) string {
	if level <= 0 {
		return ""
	}
	indent := ""
	for range level {
		indent += "    "
	}
	return indent
}
