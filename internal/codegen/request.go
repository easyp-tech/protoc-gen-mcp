package codegen

import (
	"path"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const syntheticGoPackageName = "mcpgenerated"

// PrepareRequestForProtogen adjusts the raw protoc request before protogen
// validates Go-specific metadata. Python, JVM, and TypeScript generation do not use Go
// import paths, but protogen still requires them to build its descriptor model.
func PrepareRequestForProtogen(req *pluginpb.CodeGeneratorRequest, opts Options) {
	if req == nil || !requiresSyntheticGoPackage(opts.Language) {
		return
	}

	for _, file := range req.ProtoFile {
		if file.GetOptions().GetGoPackage() != "" {
			continue
		}
		if file.Options == nil {
			file.Options = &descriptorpb.FileOptions{}
		}
		file.Options.GoPackage = proto.String(syntheticGoPackage(file, opts.Language))
	}
}

func requiresSyntheticGoPackage(language Language) bool {
	switch language {
	case LanguagePython, LanguageKotlin, LanguageJava, LanguageTypeScript:
		return true
	default:
		return false
	}
}

func syntheticGoPackage(file *descriptorpb.FileDescriptorProto, language Language) string {
	name := strings.TrimSuffix(file.GetName(), ".proto")
	name = strings.Trim(path.Clean(name), "/.")
	if name == "" {
		name = "unknown"
	}

	return "protoc-gen-mcp.local/" + string(language) + "/" + sanitizeSyntheticGoImportPath(name) + ";" + syntheticGoPackageName
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
