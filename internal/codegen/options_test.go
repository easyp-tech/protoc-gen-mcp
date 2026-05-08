package codegen

import "testing"

func TestParseOptions_DefaultLangGo(t *testing.T) {
	opts, err := ParseOptions("")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguageGo {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguageGo)
	}
	if opts.PythonRuntime != "" {
		t.Fatalf("PythonRuntime = %q, want empty", opts.PythonRuntime)
	}
	if opts.PythonHandler != "" {
		t.Fatalf("PythonHandler = %q, want empty", opts.PythonHandler)
	}
}

func TestParseOptions_JVMLanguages(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Language
	}{
		{
			name: "kotlin",
			raw:  "lang=kotlin",
			want: LanguageKotlin,
		},
		{
			name: "java",
			raw:  "lang=java",
			want: LanguageJava,
		},
		{
			name: "typescript",
			raw:  "lang=typescript",
			want: LanguageTypeScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := ParseOptions(tt.raw)
			if err != nil {
				t.Fatalf("ParseOptions returned error: %v", err)
			}

			if opts.Language != tt.want {
				t.Fatalf("Language = %q, want %q", opts.Language, tt.want)
			}
			if opts.PythonRuntime != "" {
				t.Fatalf("PythonRuntime = %q, want empty", opts.PythonRuntime)
			}
			if opts.PythonHandler != "" {
				t.Fatalf("PythonHandler = %q, want empty", opts.PythonHandler)
			}
		})
	}
}

func TestParseOptions_PythonDefaultsRuntime(t *testing.T) {
	opts, err := ParseOptions("lang=python")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonRuntime != PythonRuntimeGoogleProtobuf {
		t.Fatalf("PythonRuntime = %q, want %q", opts.PythonRuntime, PythonRuntimeGoogleProtobuf)
	}
}

func TestParseOptions_PythonDefaultsHandler(t *testing.T) {
	opts, err := ParseOptions("lang=python")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonHandler != PythonHandlerDataclass {
		t.Fatalf("PythonHandler = %q, want %q", opts.PythonHandler, PythonHandlerDataclass)
	}
}

func TestParseOptions_PythonExplicitGoogleProtobufRuntime(t *testing.T) {
	opts, err := ParseOptions("lang=python,python_runtime=google.protobuf")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonRuntime != PythonRuntimeGoogleProtobuf {
		t.Fatalf("PythonRuntime = %q, want %q", opts.PythonRuntime, PythonRuntimeGoogleProtobuf)
	}
}

func TestParseOptions_PythonExplicitDataclassHandler(t *testing.T) {
	opts, err := ParseOptions("lang=python,python_handler=dataclass")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonHandler != PythonHandlerDataclass {
		t.Fatalf("PythonHandler = %q, want %q", opts.PythonHandler, PythonHandlerDataclass)
	}
}

func TestParseOptions_PythonExplicitProtobufHandler(t *testing.T) {
	opts, err := ParseOptions("lang=python,python_runtime=google.protobuf,python_handler=protobuf")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonRuntime != PythonRuntimeGoogleProtobuf {
		t.Fatalf("PythonRuntime = %q, want %q", opts.PythonRuntime, PythonRuntimeGoogleProtobuf)
	}
	if opts.PythonHandler != PythonHandlerProtobuf {
		t.Fatalf("PythonHandler = %q, want %q", opts.PythonHandler, PythonHandlerProtobuf)
	}
}

func TestParseOptions_PythonRuntimeConstants(t *testing.T) {
	if PythonRuntimeGoogleProtobuf != "google.protobuf" {
		t.Fatalf("PythonRuntimeGoogleProtobuf = %q, want %q", PythonRuntimeGoogleProtobuf, "google.protobuf")
	}
	if PythonRuntimeBetterproto != "betterproto" {
		t.Fatalf("PythonRuntimeBetterproto = %q, want %q", PythonRuntimeBetterproto, "betterproto")
	}
	if PythonRuntimeGrpclib != "grpclib" {
		t.Fatalf("PythonRuntimeGrpclib = %q, want %q", PythonRuntimeGrpclib, "grpclib")
	}
}

func TestParseOptions_PythonHandlerConstants(t *testing.T) {
	if PythonHandlerDataclass != "dataclass" {
		t.Fatalf("PythonHandlerDataclass = %q, want %q", PythonHandlerDataclass, "dataclass")
	}
	if PythonHandlerProtobuf != "protobuf" {
		t.Fatalf("PythonHandlerProtobuf = %q, want %q", PythonHandlerProtobuf, "protobuf")
	}
}

func TestParseOptions_PythonRuntimeRejectedForNonPythonLanguages(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "go",
			raw:  "lang=go,python_runtime=google.protobuf",
		},
		{
			name: "kotlin",
			raw:  "lang=kotlin,python_runtime=google.protobuf",
		},
		{
			name: "java",
			raw:  "lang=java,python_runtime=google.protobuf",
		},
		{
			name: "typescript",
			raw:  "lang=typescript,python_runtime=google.protobuf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOptions(tt.raw)
			if err == nil {
				t.Fatal("ParseOptions succeeded, want error")
			}
		})
	}
}

func TestParseOptions_RejectsUnsupportedPythonRuntime(t *testing.T) {
	tests := []struct {
		name    string
		runtime PythonRuntime
	}{
		{
			name:    "betterproto",
			runtime: PythonRuntimeBetterproto,
		},
		{
			name:    "grpclib",
			runtime: PythonRuntimeGrpclib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOptions("lang=python,python_runtime=" + string(tt.runtime))
			if err == nil {
				t.Fatal("ParseOptions succeeded, want error")
			}
		})
	}
}

func TestParseOptions_PythonHandlerRejectedForNonPythonLanguages(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "go",
			raw:  "lang=go,python_handler=protobuf",
		},
		{
			name: "kotlin",
			raw:  "lang=kotlin,python_handler=protobuf",
		},
		{
			name: "java",
			raw:  "lang=java,python_handler=protobuf",
		},
		{
			name: "typescript",
			raw:  "lang=typescript,python_handler=protobuf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOptions(tt.raw)
			if err == nil {
				t.Fatal("ParseOptions succeeded, want error")
			}
			if got, want := err.Error(), "python_handler is only supported when lang=python"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func TestParseOptions_RejectsUnsupportedPythonHandler(t *testing.T) {
	_, err := ParseOptions("lang=python,python_handler=raw")
	if err == nil {
		t.Fatal("ParseOptions succeeded, want error")
	}
	if got, want := err.Error(), `unsupported python_handler "raw"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseOptions_IgnoresNonMCPParams(t *testing.T) {
	opts, err := ParseOptions("paths=source_relative,module=example.com/project,Mfoo.proto=example.com/project/foo,apilevelMbar.proto=API_OPEN,lang=python")
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Language != LanguagePython {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguagePython)
	}
	if opts.PythonRuntime != PythonRuntimeGoogleProtobuf {
		t.Fatalf("PythonRuntime = %q, want %q", opts.PythonRuntime, PythonRuntimeGoogleProtobuf)
	}
	if opts.PythonHandler != PythonHandlerDataclass {
		t.Fatalf("PythonHandler = %q, want %q", opts.PythonHandler, PythonHandlerDataclass)
	}
}

func TestOptionsParserSet_IgnoresProtogenManagedParams(t *testing.T) {
	parser := NewOptionsParser()

	params := []struct {
		name  string
		value string
	}{
		{name: "paths", value: "source_relative"},
		{name: "module", value: "example.com/project"},
		{name: "Mfoo.proto", value: "example.com/project/foo"},
		{name: "apilevelMbar.proto", value: "API_OPEN"},
	}

	for _, param := range params {
		if err := parser.Set(param.name, param.value); err != nil {
			t.Fatalf("Set(%q, %q) returned error: %v", param.name, param.value, err)
		}
	}

	opts, err := parser.Options()
	if err != nil {
		t.Fatalf("Options returned error: %v", err)
	}

	if opts.Language != LanguageGo {
		t.Fatalf("Language = %q, want %q", opts.Language, LanguageGo)
	}
	if opts.PythonRuntime != "" {
		t.Fatalf("PythonRuntime = %q, want empty", opts.PythonRuntime)
	}
	if opts.PythonHandler != "" {
		t.Fatalf("PythonHandler = %q, want empty", opts.PythonHandler)
	}
}

func TestParseOptions_RejectsUnknownLang(t *testing.T) {
	_, err := ParseOptions("lang=ruby")
	if err == nil {
		t.Fatal("ParseOptions succeeded, want error")
	}
}

func TestParseOptions_RejectsUnknownCustomParam(t *testing.T) {
	_, err := ParseOptions("lang=python,python_runtme=google.protobuf")
	if err == nil {
		t.Fatal("ParseOptions succeeded, want error")
	}
}
