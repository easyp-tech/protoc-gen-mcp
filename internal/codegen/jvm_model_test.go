package codegen

import (
	"reflect"
	"strings"
	"testing"
)

func TestJVMModel_StructuralIRDoesNotContainRawProtogenDescriptors(t *testing.T) {
	disallowed := map[string]struct{}{
		"google.golang.org/protobuf/compiler/protogen.Enum":    {},
		"google.golang.org/protobuf/compiler/protogen.Field":   {},
		"google.golang.org/protobuf/compiler/protogen.File":    {},
		"google.golang.org/protobuf/compiler/protogen.Message": {},
		"google.golang.org/protobuf/compiler/protogen.Method":  {},
		"google.golang.org/protobuf/compiler/protogen.Service": {},
	}

	assertIRTypeHasNoDisallowedFields(t, reflect.TypeOf(JVMFileModel{}), disallowed)
}

func TestJVMModel_StructuralIRDoesNotContainSDKTypes(t *testing.T) {
	assertIRTypeHasNoPackageSubstring(t, reflect.TypeOf(JVMFileModel{}), "modelcontextprotocol")
}

func assertIRTypeHasNoPackageSubstring(t *testing.T, typ reflect.Type, disallowed string) {
	t.Helper()

	visited := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(current reflect.Type) {
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		switch current.Kind() {
		case reflect.Map:
			walk(current.Key())
			walk(current.Elem())
			return
		case reflect.Struct:
		default:
			return
		}

		if visited[current] {
			return
		}
		visited[current] = true

		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			fieldType := field.Type
			baseType := fieldType
			for baseType.Kind() == reflect.Pointer || baseType.Kind() == reflect.Slice || baseType.Kind() == reflect.Array {
				baseType = baseType.Elem()
			}
			if strings.Contains(baseType.PkgPath(), disallowed) {
				t.Fatalf("%s.%s uses disallowed package %q in field type %s", current.Name(), field.Name, disallowed, fieldType)
			}
			walk(fieldType)
		}
	}

	walk(typ)
}
