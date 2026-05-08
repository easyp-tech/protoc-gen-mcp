package codegen

import (
	"fmt"
	"strings"
)

type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageKotlin     Language = "kotlin"
	LanguageJava       Language = "java"
	LanguageTypeScript Language = "typescript"
)

type PythonRuntime string

const (
	PythonRuntimeGoogleProtobuf PythonRuntime = "google.protobuf"
	PythonRuntimeBetterproto    PythonRuntime = "betterproto"
	PythonRuntimeGrpclib        PythonRuntime = "grpclib"
)

type PythonHandler string

const PythonHandlerDataclass PythonHandler = "dataclass"
const PythonHandlerProtobuf PythonHandler = "protobuf"

type Options struct {
	Language      Language
	PythonRuntime PythonRuntime
	PythonHandler PythonHandler
}

type OptionsParser struct {
	opts             Options
	sawPythonRuntime bool
	sawPythonHandler bool
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
	case "python_handler":
		p.opts.PythonHandler = PythonHandler(value)
		p.sawPythonHandler = true
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
		if p.sawPythonHandler {
			return Options{}, fmt.Errorf("python_handler is only supported when lang=python")
		}
	case LanguageKotlin, LanguageJava, LanguageTypeScript:
		if p.sawPythonRuntime {
			return Options{}, fmt.Errorf("python_runtime is only supported when lang=python")
		}
		if p.sawPythonHandler {
			return Options{}, fmt.Errorf("python_handler is only supported when lang=python")
		}
	case LanguagePython:
		if !p.sawPythonRuntime {
			p.opts.PythonRuntime = PythonRuntimeGoogleProtobuf
		}
		if !p.sawPythonHandler {
			p.opts.PythonHandler = PythonHandlerDataclass
		}
		switch p.opts.PythonRuntime {
		case PythonRuntimeGoogleProtobuf:
		default:
			return Options{}, fmt.Errorf("unsupported python_runtime %q", p.opts.PythonRuntime)
		}
		switch p.opts.PythonHandler {
		case PythonHandlerDataclass, PythonHandlerProtobuf:
		default:
			return Options{}, fmt.Errorf("unsupported python_handler %q", p.opts.PythonHandler)
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
