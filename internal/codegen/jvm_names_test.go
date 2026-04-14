package codegen

import (
	"strings"
	"testing"
)

func TestJVMNames_GeneratedAPINames(t *testing.T) {
	if got, want := jvmMethodName("CreateReport"), "createReport"; got != want {
		t.Fatalf("jvmMethodName = %q, want %q", got, want)
	}
	if got, want := jvmHandlerName("ExampleAPI"), "ExampleAPIToolHandler"; got != want {
		t.Fatalf("jvmHandlerName = %q, want %q", got, want)
	}
	if got, want := jvmRegisterName("ExampleAPI"), "registerExampleAPITools"; got != want {
		t.Fatalf("jvmRegisterName = %q, want %q", got, want)
	}
	if got, want := jvmSchemaConst("ExampleAPI", "CreateReport"), "EXAMPLE_API_CREATE_REPORT"; got != want {
		t.Fatalf("jvmSchemaConst = %q, want %q", got, want)
	}
	if got, want := jvmOneofWrapperName("CreateReportRequest", "city"), "CreateReportRequestCityVariant"; got != want {
		t.Fatalf("jvmOneofWrapperName = %q, want %q", got, want)
	}
	if got, want := jvmOneofVariantWrapperName("CreateReportRequest", "city", "city_id"), "CreateReportRequestCityCityIdVariant"; got != want {
		t.Fatalf("jvmOneofVariantWrapperName = %q, want %q", got, want)
	}
}

func TestJVMNames_NormalizesProtoPath(t *testing.T) {
	if got, want := jvmGeneratedFilenamePrefixForProtoPath("test/v1/my-file.proto"), "test/v1/my_file"; got != want {
		t.Fatalf("jvmGeneratedFilenamePrefixForProtoPath = %q, want %q", got, want)
	}
}

func TestJVMNames_FailsOnCollision(t *testing.T) {
	err := jvmCheckNameCollision(map[string]string{
		"Report": "test.v1.Report",
	}, "public type name", "Report", "other.v1.Report")
	if err == nil {
		t.Fatal("jvmCheckNameCollision unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "jvm") {
		t.Fatalf("collision error = %q, want jvm diagnostic", err)
	}

	scoped := map[string]map[string]string{}
	if err := jvmCheckOwnedNameCollision(scoped, "test/v1/report.proto", "public type name", "Report", "test.v1.Report"); err != nil {
		t.Fatalf("jvmCheckOwnedNameCollision first insert: %v", err)
	}
	err = jvmCheckOwnedNameCollision(scoped, "test/v1/report.proto", "public type name", "Report", "test.v1.OtherReport")
	if err == nil {
		t.Fatal("jvmCheckOwnedNameCollision unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "jvm") {
		t.Fatalf("owned collision error = %q, want jvm diagnostic", err)
	}
}
