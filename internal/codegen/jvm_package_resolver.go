package codegen

import (
	"path"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type jvmResolvedFilePackage struct {
	Package        string
	MultipleFiles  bool
	OuterClassName string
}

type jvmResolvedTypeRef struct {
	Package       string
	ImportPath    string
	Expr          string
	IsCurrentFile bool
}

func resolveJVMFilePackage(file *protogen.File) (jvmResolvedFilePackage, error) {
	return resolveJVMDescriptorFilePackage(file.Desc)
}

func resolveJVMDescriptorFilePackage(file protoreflect.FileDescriptor) (jvmResolvedFilePackage, error) {
	options, _ := file.Options().(*descriptorpb.FileOptions)
	javaPackage := strings.TrimSpace(options.GetJavaPackage())
	if javaPackage == "" {
		javaPackage = string(file.Package())
	}

	outerClassName := strings.TrimSpace(options.GetJavaOuterClassname())
	if outerClassName == "" {
		outerClassName = defaultJVMOuterClassName(file.Path())
	}

	return jvmResolvedFilePackage{
		Package:        javaPackage,
		MultipleFiles:  options.GetJavaMultipleFiles(),
		OuterClassName: outerClassName,
	}, nil
}

func resolveJVMMessageTypeRef(message *protogen.Message, currentProtoPath string) (jvmResolvedTypeRef, error) {
	if message == nil {
		return jvmResolvedTypeRef{}, nil
	}
	return resolveJVMTypeRef(message.Desc.ParentFile(), jvmPublicTypeName(message), currentProtoPath)
}

func resolveJVMEnumTypeRef(enum *protogen.Enum, currentProtoPath string) (jvmResolvedTypeRef, error) {
	if enum == nil {
		return jvmResolvedTypeRef{}, nil
	}
	return resolveJVMTypeRef(enum.Desc.ParentFile(), jvmPublicEnumName(enum), currentProtoPath)
}

func resolveJVMTypeRef(file protoreflect.FileDescriptor, publicName, currentProtoPath string) (jvmResolvedTypeRef, error) {
	resolvedFile, err := resolveJVMDescriptorFilePackage(file)
	if err != nil {
		return jvmResolvedTypeRef{}, err
	}

	isCurrentFile := file.Path() == currentProtoPath
	expr := publicName
	importPath := ""
	if resolvedFile.MultipleFiles {
		if !isCurrentFile {
			importPath = resolvedFile.Package + "." + publicName
		}
	} else {
		expr = resolvedFile.OuterClassName + "." + publicName
		if !isCurrentFile {
			importPath = resolvedFile.Package + "." + resolvedFile.OuterClassName
		}
	}

	return jvmResolvedTypeRef{
		Package:       resolvedFile.Package,
		ImportPath:    importPath,
		Expr:          expr,
		IsCurrentFile: isCurrentFile,
	}, nil
}

func defaultJVMOuterClassName(protoPath string) string {
	base := strings.TrimSuffix(path.Base(protoPath), path.Ext(protoPath))
	var b strings.Builder
	nextUpper := true
	for _, r := range base {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			nextUpper = true
			continue
		}
		if nextUpper {
			b.WriteRune(unicode.ToUpper(r))
			nextUpper = false
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "ProtoOuterClass"
	}
	b.WriteString("OuterClass")
	return b.String()
}
