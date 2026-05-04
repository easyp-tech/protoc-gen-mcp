package codegen

import (
	"strings"
	"testing"
)

func TestTypeScriptNames_GeneratedAPINames(t *testing.T) {
	if got, want := typescriptMethodName("CreateReport"), "createReport"; got != want {
		t.Fatalf("typescriptMethodName = %q, want %q", got, want)
	}
	if got, want := typescriptHandlerName("ExampleAPI"), "ExampleAPIToolHandler"; got != want {
		t.Fatalf("typescriptHandlerName = %q, want %q", got, want)
	}
	if got, want := typescriptRegisterName("ExampleAPI"), "registerExampleAPITools"; got != want {
		t.Fatalf("typescriptRegisterName = %q, want %q", got, want)
	}
	if got, want := typescriptSchemaConst("ExampleAPI", "CreateReport"), "EXAMPLE_API_CREATE_REPORT"; got != want {
		t.Fatalf("typescriptSchemaConst = %q, want %q", got, want)
	}
}

func TestTypeScriptNames_ProtobufModuleSpecifiersAreNodeNextRelative(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current string
		target  string
		want    string
	}{
		{
			name:    "same directory",
			current: "test/v1/service-file.proto",
			target:  "test/v1/shared-file.proto",
			want:    "./shared_file_pb.js",
		},
		{
			name:    "parent directory",
			current: "test/v1/deep/service.proto",
			target:  "test/v1/shared-file.proto",
			want:    "../shared_file_pb.js",
		},
		{
			name:    "sibling branch",
			current: "test/v1/deep/service.proto",
			target:  "test/common/shared-file.proto",
			want:    "../../common/shared_file_pb.js",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := typescriptProtobufModuleSpecifier(tc.current, tc.target)
			if got != tc.want {
				t.Fatalf("typescriptProtobufModuleSpecifier = %q, want %q", got, tc.want)
			}
			if !strings.HasSuffix(got, ".js") {
				t.Fatalf("module specifier = %q, want .js extension", got)
			}
			if !strings.HasPrefix(got, "./") && !strings.HasPrefix(got, "../") {
				t.Fatalf("module specifier = %q, want relative prefix", got)
			}
			if strings.Contains(got, `\`) {
				t.Fatalf("module specifier = %q, want POSIX separators", got)
			}
		})
	}
}

func TestTypeScriptNames_ProtobufESRefs(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/ref-file.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/reffile;reffilev1";`,
			`message Report {`,
			`  message Entry {}`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/ref-file.proto")
	file := plugin.FilesByPath["test/v1/ref-file.proto"]
	if file == nil {
		t.Fatal("ref-file proto file not found in plugin")
	}
	report := file.Messages[0]
	entry := report.Messages[0]

	if got, want := typescriptPublicTypeName(report), "Report"; got != want {
		t.Fatalf("typescriptPublicTypeName(report) = %q, want %q", got, want)
	}
	if got, want := typescriptPublicTypeName(entry), "Report_Entry"; got != want {
		t.Fatalf("typescriptPublicTypeName(entry) = %q, want %q", got, want)
	}
	if got, want := typescriptSchemaName(typescriptPublicTypeName(entry)), "Report_EntrySchema"; got != want {
		t.Fatalf("typescriptSchemaName(entry) = %q, want %q", got, want)
	}
	if got, want := typescriptSchemaName("Report"), "ReportSchema"; got != want {
		t.Fatalf("typescriptSchemaName = %q, want %q", got, want)
	}
	if got, want := typescriptFileRegistryRefName("test/v1/ref-file.proto"), "file_test_v1_ref_file"; got != want {
		t.Fatalf("typescriptFileRegistryRefName = %q, want %q", got, want)
	}
	if got, want := typescriptRegistryRefForProtoPath("test/v1/ref-file.proto").RefName, "file_test_v1_ref_file"; got != want {
		t.Fatalf("RegistryRef.RefName = %q, want %q", got, want)
	}
}

func TestTypeScriptNames_FailsOnCollision(t *testing.T) {
	err := typescriptCheckNameCollision(map[string]string{
		"Report": "test/v1/a.proto",
	}, "import type name", "Report", "test/v1/b.proto")
	if err == nil {
		t.Fatal("typescriptCheckNameCollision unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "typescript") || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("collision error = %q, want typescript collision diagnostic", err)
	}

	scoped := map[string]map[string]string{}
	if err := typescriptCheckOwnedNameCollision(scoped, "test/v1/a.proto", "import type name", "Report", "test.v1.Report"); err != nil {
		t.Fatalf("typescriptCheckOwnedNameCollision first insert: %v", err)
	}
	err = typescriptCheckOwnedNameCollision(scoped, "test/v1/a.proto", "import type name", "Report", "test.v1.OtherReport")
	if err == nil {
		t.Fatal("typescriptCheckOwnedNameCollision unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "typescript") {
		t.Fatalf("owned collision error = %q, want typescript diagnostic", err)
	}

	err = validateTypeScriptImportCollisions([]TypeScriptImport{
		{
			ProtoPath:   "test/v1/a.proto",
			TypeNames:   []string{"Report"},
			SchemaNames: []string{"ReportSchema"},
		},
		{
			ProtoPath:   "test/v1/b.proto",
			TypeNames:   []string{"Report"},
			SchemaNames: []string{"ReportSchema"},
		},
	})
	if err == nil {
		t.Fatal("validateTypeScriptImportCollisions unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "typescript") || !strings.Contains(err.Error(), "Report") {
		t.Fatalf("import collision error = %q, want TypeScript import symbol diagnostic", err)
	}
}
