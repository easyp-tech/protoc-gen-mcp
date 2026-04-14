package codegen

import (
	"path"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const pythonSyntheticGoPackageName = "mcppython"

// PrepareRequestForProtogen adjusts the raw protoc request before protogen
// validates Go-specific metadata. Python generation does not use Go import
// paths, but protogen still requires them to build its descriptor model.
func PrepareRequestForProtogen(req *pluginpb.CodeGeneratorRequest, opts Options) {
	if req == nil || opts.Language != LanguagePython {
		return
	}

	for _, file := range req.ProtoFile {
		if file.GetOptions().GetGoPackage() != "" {
			continue
		}
		if file.Options == nil {
			file.Options = &descriptorpb.FileOptions{}
		}
		file.Options.GoPackage = proto.String(syntheticPythonGoPackage(file))
	}
}

func syntheticPythonGoPackage(file *descriptorpb.FileDescriptorProto) string {
	name := strings.TrimSuffix(file.GetName(), ".proto")
	name = strings.Trim(path.Clean(name), "/.")
	if name == "" {
		name = "unknown"
	}

	return "protoc-gen-mcp.local/python/" + sanitizeSyntheticGoImportPath(name) + ";" + pythonSyntheticGoPackageName
}

func sanitizeSyntheticGoImportPath(value string) string {
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '/', char == '.', char == '_', char == '-':
			builder.WriteRune(char)
		default:
			builder.WriteByte('_')
		}
	}

	sanitized := strings.Trim(builder.String(), "/.")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}
