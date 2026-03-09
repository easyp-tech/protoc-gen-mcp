package mcpruntime

import "strings"

// RegisterOptions configures generated tool registration.
type RegisterOptions struct {
	Namespace string
}

// RegisterOption mutates RegisterOptions.
type RegisterOption func(*RegisterOptions)

// WithNamespace overrides the generated tool namespace.
func WithNamespace(namespace string) RegisterOption {
	return func(options *RegisterOptions) {
		options.Namespace = strings.TrimSpace(namespace)
	}
}

func resolveOptions(defaultNamespace string, options []RegisterOption) RegisterOptions {
	resolved := RegisterOptions{
		Namespace: strings.TrimSpace(defaultNamespace),
	}

	for _, option := range options {
		if option != nil {
			option(&resolved)
		}
	}

	return resolved
}

func qualifyToolName(namespace, name string) string {
	namespace = normalizeToolSegment(namespace)
	name = normalizeToolSegment(name)
	if namespace == "" {
		return name
	}
	if name == "" {
		return namespace
	}

	return namespace + "_" + name
}

func normalizeToolSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}

	parts := strings.FieldsFunc(segment, func(r rune) bool {
		return r == '.' || r == '_'
	})
	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "_")
}
