package codegen

import (
	"fmt"
	"strings"
)

type Language string

const (
	LanguageGo     Language = "go"
	LanguagePython Language = "python"
)

type PythonRuntime string

const (
	PythonRuntimeGoogleProtobuf PythonRuntime = "google.protobuf"
	PythonRuntimeBetterproto    PythonRuntime = "betterproto"
	PythonRuntimeGrpclib        PythonRuntime = "grpclib"
)

type Options struct {
	Language      Language
	PythonRuntime PythonRuntime
}

type OptionsParser struct {
	opts             Options
	sawPythonRuntime bool
}

func NewOptionsParser() *OptionsParser {
	return &OptionsParser{
		opts: Options{
			Language: LanguageGo,
		},
	}
}

func (p *OptionsParser) Set(name, value string) error {
	if isProtogenManagedParam(name) {
		return nil
	}

	switch name {
	case "lang":
		p.opts.Language = Language(value)
	case "python_runtime":
		p.opts.PythonRuntime = PythonRuntime(value)
		p.sawPythonRuntime = true
	default:
		return fmt.Errorf("unknown protoc-gen-mcp option %q", name)
	}

	return nil
}

func (p *OptionsParser) Options() (Options, error) {
	switch p.opts.Language {
	case LanguageGo:
		if p.sawPythonRuntime {
			return Options{}, fmt.Errorf("python_runtime is only supported when lang=python")
		}
	case LanguagePython:
		if !p.sawPythonRuntime {
			p.opts.PythonRuntime = PythonRuntimeGoogleProtobuf
		}
		switch p.opts.PythonRuntime {
		case PythonRuntimeGoogleProtobuf:
		default:
			return Options{}, fmt.Errorf("unsupported python_runtime %q", p.opts.PythonRuntime)
		}
	default:
		return Options{}, fmt.Errorf("unsupported lang %q", p.opts.Language)
	}

	return p.opts, nil
}

// ParseOptions exists for direct parser tests and any non-protogen callers that
// need the same option semantics as the plugin entrypoint.
func ParseOptions(raw string) (Options, error) {
	parser := NewOptionsParser()

	for _, param := range strings.Split(raw, ",") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}

		name, value, hasValue := strings.Cut(param, "=")
		if !hasValue {
			value = ""
		}

		if isProtogenManagedParam(name) {
			continue
		}

		if err := parser.Set(name, value); err != nil {
			return Options{}, err
		}
	}

	return parser.Options()
}

func isProtogenManagedParam(name string) bool {
	switch {
	case name == "":
		return true
	case name == "module", name == "paths", name == "annotate_code", name == "default_api_level":
		return true
	case strings.HasPrefix(name, "M"):
		return true
	case strings.HasPrefix(name, "apilevelM"):
		return true
	default:
		return false
	}
}
