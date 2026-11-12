package codegen

import "strings"

// URI template parameter names only have to match [a-zA-Z_][a-zA-Z0-9_]*, so
// a template such as "files://{type}/{id}" can carry a name that is reserved
// in a target language. Renderers use these helpers for the generated
// identifier; the original name stays the key on the wire.

var goReservedParamNames = setOf(
	"break", "case", "chan", "const", "continue", "default", "defer", "else",
	"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch", "type",
	"var",
	// The generated Read method already takes ctx as its first parameter.
	"ctx",
)

var pythonReservedParamNames = setOf(
	"False", "None", "True", "and", "as", "assert", "async", "await", "break",
	"class", "continue", "def", "del", "elif", "else", "except", "finally",
	"for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal",
	"not", "or", "pass", "raise", "return", "try", "while", "with", "yield",
	// The generated protocol methods already take self.
	"self",
)

var typescriptReservedParamNames = setOf(
	"break", "case", "catch", "class", "const", "continue", "debugger",
	"default", "delete", "do", "else", "enum", "export", "extends", "false",
	"finally", "for", "function", "if", "import", "in", "instanceof", "new",
	"null", "return", "super", "switch", "this", "throw", "true", "try",
	"typeof", "var", "void", "while", "with",
	// Reserved in strict mode, which ES modules always use.
	"implements", "interface", "let", "package", "private", "protected",
	"public", "static", "yield", "await", "arguments", "eval",
)

var kotlinReservedParamNames = setOf(
	"as", "break", "class", "continue", "do", "else", "false", "for", "fun",
	"if", "in", "interface", "is", "null", "object", "package", "return",
	"super", "this", "throw", "true", "try", "typealias", "typeof", "val",
	"var", "when", "while",
)

var javaReservedParamNames = setOf(
	"abstract", "assert", "boolean", "break", "byte", "case", "catch", "char",
	"class", "const", "continue", "default", "do", "double", "else", "enum",
	"extends", "final", "finally", "float", "for", "goto", "if", "implements",
	"import", "instanceof", "int", "interface", "long", "native", "new",
	"package", "private", "protected", "public", "return", "short", "static",
	"strictfp", "super", "switch", "synchronized", "this", "throw", "throws",
	"transient", "try", "void", "volatile", "while", "true", "false", "null",
	"_",
)

func setOf(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// isUnderscoreOnly reports names such as "_" or "__", which Kotlin reserves.
func isUnderscoreOnly(name string) bool {
	return name != "" && strings.Trim(name, "_") == ""
}

func goParamIdentifiers(params []ResourceParamModel) []string {
	return safeParamIdentifiers(params, func(name string) bool {
		_, ok := goReservedParamNames[name]
		return ok
	})
}

func pythonParamIdentifiers(params []ResourceParamModel) []string {
	return safeParamIdentifiers(params, func(name string) bool {
		_, ok := pythonReservedParamNames[name]
		return ok
	})
}

func typescriptParamIdentifiers(params []ResourceParamModel) []string {
	return safeParamIdentifiers(params, func(name string) bool {
		_, ok := typescriptReservedParamNames[name]
		return ok
	})
}

func kotlinParamIdentifiers(params []ResourceParamModel) []string {
	return safeParamIdentifiers(params, func(name string) bool {
		_, ok := kotlinReservedParamNames[name]
		return ok || isUnderscoreOnly(name)
	})
}

func javaParamIdentifiers(params []ResourceParamModel) []string {
	return safeParamIdentifiers(params, func(name string) bool {
		_, ok := javaReservedParamNames[name]
		return ok
	})
}

// safeParamIdentifiers maps each parameter to an identifier that is neither
// reserved nor already taken by another parameter of the same signature.
// Reserved names get a trailing underscore; names that stay reserved after
// that (underscore-only names in Kotlin) get a "param" prefix instead.
func safeParamIdentifiers(params []ResourceParamModel, reserved func(string) bool) []string {
	taken := make(map[string]struct{}, len(params))
	for _, param := range params {
		if !reserved(param.Name) {
			taken[param.Name] = struct{}{}
		}
	}

	identifiers := make([]string, len(params))
	for i, param := range params {
		identifier := param.Name
		if reserved(identifier) {
			identifier += "_"
			if reserved(identifier) {
				identifier = "param" + param.Name
			}
			for {
				if _, exists := taken[identifier]; !exists && !reserved(identifier) {
					break
				}
				identifier += "_"
			}
			taken[identifier] = struct{}{}
		}
		identifiers[i] = identifier
	}
	return identifiers
}
