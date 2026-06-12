package mcpruntime

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ParsePromptArguments fills a proto message from MCP prompt arguments.
// Scalar fields are parsed from string values. Enum fields are matched by name.
// Returns an error for missing required arguments or parse failures.
func ParsePromptArguments(args map[string]string, msg proto.Message, requiredFields []string) error {
	// Check required fields.
	for _, name := range requiredFields {
		if _, ok := args[name]; !ok {
			return fmt.Errorf("missing required argument %q", name)
		}
	}

	md := msg.ProtoReflect().Descriptor()
	fields := md.Fields()
	msgRef := msg.ProtoReflect()

	for key, value := range args {
		// Find field by JSON name first, then by proto name.
		var fd protoreflect.FieldDescriptor
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			if f.JSONName() == key || string(f.Name()) == key {
				fd = f
				break
			}
		}
		if fd == nil {
			// Unknown argument — skip (DiscardUnknown semantics).
			continue
		}

		pv, err := parseScalarValue(fd, value)
		if err != nil {
			return fmt.Errorf("invalid value %q for argument %q: %w", value, key, err)
		}
		msgRef.Set(fd, pv)
	}

	return nil
}

func parseScalarValue(fd protoreflect.FieldDescriptor, value string) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(value), nil

	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(b), nil

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(n)), nil

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(n), nil

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(n)), nil

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(n), nil

	case protoreflect.FloatKind:
		f, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat32(float32(f)), nil

	case protoreflect.DoubleKind:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat64(f), nil

	case protoreflect.BytesKind:
		b, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBytes(b), nil

	case protoreflect.EnumKind:
		enumDesc := fd.Enum()
		ev := enumDesc.Values().ByName(protoreflect.Name(value))
		if ev == nil {
			return protoreflect.Value{}, fmt.Errorf("unknown enum value %q for enum %s", value, enumDesc.FullName())
		}
		return protoreflect.ValueOfEnum(ev.Number()), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind %s", fd.Kind())
	}
}
