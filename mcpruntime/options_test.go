package mcpruntime

import "testing"

func TestQualifyToolName(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
		tool      string
		want      string
	}{
		{
			name:      "namespace prefix uses underscore separator",
			namespace: "example",
			tool:      "CreateReport",
			want:      "example_CreateReport",
		},
		{
			name:      "dotted namespace is normalized",
			namespace: "example.v1",
			tool:      "CreateReport",
			want:      "example_v1_CreateReport",
		},
		{
			name:      "dotted tool name is normalized",
			namespace: "example",
			tool:      "Health.Check",
			want:      "example_Health_Check",
		},
		{
			name:      "empty namespace keeps tool name",
			namespace: "",
			tool:      "Health",
			want:      "Health",
		},
		{
			name:      "redundant separators collapse",
			namespace: " example._v1. ",
			tool:      " _Health_. ",
			want:      "example_v1_Health",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := qualifyToolName(testCase.namespace, testCase.tool)
			if got != testCase.want {
				t.Fatalf("qualifyToolName(%q, %q) = %q, want %q", testCase.namespace, testCase.tool, got, testCase.want)
			}
		})
	}
}
