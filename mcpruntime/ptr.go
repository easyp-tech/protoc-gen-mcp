package mcpruntime

// Ptr returns a pointer to v. It is a helper for generated code that needs to
// populate optional (pointer) fields such as Annotations.Priority from a value.
func Ptr[T any](v T) *T {
	return &v
}
