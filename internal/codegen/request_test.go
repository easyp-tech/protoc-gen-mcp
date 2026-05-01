package codegen

import (
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestPrepareRequestForProtogen_SynthesizesGoPackageForPython(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("proto/notebook.proto"),
				Package: proto.String("notebook.v1"),
				Options: &descriptorpb.FileOptions{},
			},
		},
	}

	PrepareRequestForProtogen(req, Options{Language: LanguagePython})

	got := req.ProtoFile[0].GetOptions().GetGoPackage()
	if got == "" {
		t.Fatal("GoPackage was not synthesized")
	}
	if got == "notebook.v1" {
		t.Fatalf("GoPackage = %q, want an import path with explicit package suffix", got)
	}
}

func TestPrepareRequestForProtogen_SynthesizesGoPackageForJVM(t *testing.T) {
	for _, language := range []Language{LanguageKotlin, LanguageJava} {
		t.Run(string(language), func(t *testing.T) {
			req := &pluginpb.CodeGeneratorRequest{
				ProtoFile: []*descriptorpb.FileDescriptorProto{
					{
						Name:    proto.String("proto/notebook.proto"),
						Package: proto.String("notebook.v1"),
						Options: &descriptorpb.FileOptions{},
					},
				},
			}

			PrepareRequestForProtogen(req, Options{Language: language})

			got := req.ProtoFile[0].GetOptions().GetGoPackage()
			if got == "" {
				t.Fatal("GoPackage was not synthesized")
			}
			if got == "notebook.v1" {
				t.Fatalf("GoPackage = %q, want an import path with explicit package suffix", got)
			}
		})
	}
}

func TestPrepareRequestForProtogen_DoesNotSynthesizeGoPackageForGo(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("proto/notebook.proto"),
				Package: proto.String("notebook.v1"),
				Options: &descriptorpb.FileOptions{},
			},
		},
	}

	PrepareRequestForProtogen(req, Options{Language: LanguageGo})

	if got := req.ProtoFile[0].GetOptions().GetGoPackage(); got != "" {
		t.Fatalf("GoPackage = %q, want empty for Go mode", got)
	}
}

func TestPrepareRequestForProtogen_PreservesExplicitGoPackage(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("proto/notebook.proto"),
				Package: proto.String("notebook.v1"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String("example.com/acme/notebook;notebookv1"),
				},
			},
		},
	}

	PrepareRequestForProtogen(req, Options{Language: LanguagePython})

	if got := req.ProtoFile[0].GetOptions().GetGoPackage(); got != "example.com/acme/notebook;notebookv1" {
		t.Fatalf("GoPackage = %q, want explicit value preserved", got)
	}
}

func TestPrepareRequestForProtogen_AllowsNonGoProtogenNewWithoutGoPackage(t *testing.T) {
	for _, language := range []Language{LanguagePython, LanguageKotlin, LanguageJava} {
		t.Run(string(language), func(t *testing.T) {
			req := &pluginpb.CodeGeneratorRequest{
				Parameter:      proto.String("paths=source_relative,lang=" + string(language)),
				FileToGenerate: []string{"proto/notebook.proto"},
				ProtoFile: []*descriptorpb.FileDescriptorProto{
					{
						Name:    proto.String("proto/notebook.proto"),
						Package: proto.String("notebook.v1"),
						Syntax:  proto.String("proto3"),
						Options: &descriptorpb.FileOptions{},
					},
				},
			}
			opts, err := ParseOptions(req.GetParameter())
			if err != nil {
				t.Fatalf("ParseOptions: %v", err)
			}
			PrepareRequestForProtogen(req, opts)

			parser := NewOptionsParser()
			plugin, err := (protogen.Options{ParamFunc: parser.Set}).New(req)
			if err != nil {
				t.Fatalf("protogen.Options.New() failed after %s request preparation: %v", language, err)
			}
			if got := plugin.FilesByPath["proto/notebook.proto"].GoImportPath; got == "" {
				t.Fatal("GoImportPath is empty after request preparation")
			}
		})
	}
}
