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
	Language       Language
	PythonRuntime  PythonRuntime
	PythonHandler  PythonHandler
	PythonHandlers []PythonHandler
}

type OptionsParser struct {
	opts                           Options
	sawPythonRuntime               bool
	sawPythonHandler               bool
	allowPythonHandlerContinuation bool
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
		p.allowPythonHandlerContinuation = false
		return nil
	}

	switch name {
	case "lang":
		p.opts.Language = Language(value)
		p.allowPythonHandlerContinuation = false
	case "python_runtime":
		p.opts.PythonRuntime = PythonRuntime(value)
		p.sawPythonRuntime = true
		p.allowPythonHandlerContinuation = false
	case "python_handler":
		p.addPythonHandlerValues(value)
		p.sawPythonHandler = true
		p.allowPythonHandlerContinuation = true
	default:
		if value == "" && p.allowPythonHandlerContinuation {
			p.addPythonHandler(PythonHandler(name))
			return nil
		}
		p.allowPythonHandlerContinuation = false
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
			p.addPythonHandler(PythonHandlerDataclass)
		}
		switch p.opts.PythonRuntime {
		case PythonRuntimeGoogleProtobuf:
		default:
			return Options{}, fmt.Errorf("unsupported python_runtime %q", p.opts.PythonRuntime)
		}
		for _, handler := range p.opts.PythonHandlers {
			switch handler {
			case PythonHandlerDataclass, PythonHandlerProtobuf:
			default:
				return Options{}, fmt.Errorf("unsupported python_handler %q", handler)
			}
		}
		if len(p.opts.PythonHandlers) == 0 {
			return Options{}, fmt.Errorf(`unsupported python_handler ""`)
		}
		p.opts.PythonHandler = p.opts.PythonHandlers[0]
	default:
		return Options{}, fmt.Errorf("unsupported lang %q", p.opts.Language)
	}

	return p.opts, nil
}

func (p *OptionsParser) addPythonHandlerValues(value string) {
	for _, handler := range splitPythonHandlerValue(value) {
		p.addPythonHandler(handler)
	}
}

func (p *OptionsParser) addPythonHandler(handler PythonHandler) {
	for _, existing := range p.opts.PythonHandlers {
		if existing == handler {
			return
		}
	}
	p.opts.PythonHandlers = append(p.opts.PythonHandlers, handler)
	p.opts.PythonHandler = p.opts.PythonHandlers[0]
}

func splitPythonHandlerValue(value string) []PythonHandler {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '+' || r == '|' || r == ';'
	})
	if len(parts) == 0 {
		return []PythonHandler{PythonHandler(value)}
	}

	handlers := make([]PythonHandler, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		handlers = append(handlers, PythonHandler(part))
	}
	if len(handlers) == 0 {
		return []PythonHandler{PythonHandler(value)}
	}
	return handlers
}

func pythonHandlersForOptions(opts Options) []PythonHandler {
	if len(opts.PythonHandlers) > 0 {
		return opts.PythonHandlers
	}
	if opts.PythonHandler != "" {
		return []PythonHandler{opts.PythonHandler}
	}
	if opts.Language == LanguagePython {
		return []PythonHandler{PythonHandlerDataclass}
	}
	return nil
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
