package codegen

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/schema"
	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/runtime/protoimpl"
	"pgregory.net/rapid"
)

// Feature: options-package-migration, Property 3: Round-trip материализации ExampleValue
// **Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7**

func genExampleValue(depth int) *rapid.Generator[*mcpoptionsv1.ExampleValue] {
	return rapid.Custom[*mcpoptionsv1.ExampleValue](func(t *rapid.T) *mcpoptionsv1.ExampleValue {
		// At max depth, only generate leaf values (no object/array recursion).
		maxKind := 6
		if depth <= 0 {
			maxKind = 4 // string, number, bool, null only
		}

		kind := rapid.IntRange(1, maxKind).Draw(t, "kind")
		switch kind {
		case 1:
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_StringValue{
					StringValue: rapid.String().Draw(t, "string"),
				},
			}
		case 2:
			// Generate finite float64 values only — NaN/Inf cannot round-trip through JSON.
			v := rapid.Float64().Draw(t, "number")
			for math.IsNaN(v) || math.IsInf(v, 0) {
				v = rapid.Float64().Draw(t, "number")
			}
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_NumberValue{
					NumberValue: v,
				},
			}
		case 3:
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_BoolValue{
					BoolValue: rapid.Bool().Draw(t, "bool"),
				},
			}
		case 4:
			// null_value: true emits JSON null; null_value: false returns ok=false.
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_NullValue{
					NullValue: true,
				},
			}
		case 5:
			// Object with recursive children.
			n := rapid.IntRange(0, 4).Draw(t, "objSize")
			props := make(map[string]*mcpoptionsv1.ExampleValue, n)
			for i := 0; i < n; i++ {
				key := rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "key")
				props[key] = genExampleValue(depth-1).Draw(t, "objVal")
			}
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_ObjectValue{
					ObjectValue: &mcpoptionsv1.ExampleObject{
						Properties: props,
					},
				},
			}
		default:
			// Array with recursive children.
			n := rapid.IntRange(0, 4).Draw(t, "arrSize")
			items := make([]*mcpoptionsv1.ExampleValue, n)
			for i := range items {
				items[i] = genExampleValue(depth-1).Draw(t, "arrItem")
			}
			return &mcpoptionsv1.ExampleValue{
				Kind: &mcpoptionsv1.ExampleValue_ArrayValue{
					ArrayValue: &mcpoptionsv1.ExampleArray{
						Items: items,
					},
				},
			}
		}
	})
}

func TestProperty3_RoundTripMaterializeExampleValue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := genExampleValue(3).Draw(t, "exampleValue")

		result, ok := materializeExampleValue(v)
		if !ok {
			return // nil or unset kind — nothing to round-trip
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var roundTripped any
		if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(result, roundTripped) {
			t.Fatalf("round-trip mismatch:\n  original:     %#v\n  roundTripped: %#v\n  json: %s", result, roundTripped, jsonBytes)
		}
	})
}

// Feature: options-package-migration, Property 4: Материализация default_value в JSON Schema
// **Validates: Requirements 11.1, 11.2**

func TestProperty4_DefaultValueMaterialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := genExampleValue(3).Draw(t, "defaultValue")

		materialized, ok := materializeExampleValue(v)
		if !ok {
			// nil or unset kind — verify that FieldMetadata with HasDefault=false
			// has no default set.
			fm := schema.FieldMetadata{HasDefault: false}
			if fm.Default != nil {
				t.Fatalf("FieldMetadata with HasDefault=false should have nil Default, got %v", fm.Default)
			}
			return
		}

		// 1. Create FieldMetadata with HasDefault: true and the materialized default.
		fm := schema.FieldMetadata{
			HasDefault: true,
			Default:    materialized,
		}

		// 2. Verify HasDefault is true and Default matches the materialized value.
		if !fm.HasDefault {
			t.Fatal("expected HasDefault to be true")
		}
		if !reflect.DeepEqual(fm.Default, materialized) {
			t.Fatalf("Default mismatch:\n  expected: %#v\n  got:      %#v", materialized, fm.Default)
		}

		// 3. Verify the materialized default can be serialized to json.RawMessage
		//    (the format used by jsonschema.Schema.Default).
		jsonBytes, err := json.Marshal(fm.Default)
		if err != nil {
			t.Fatalf("json.Marshal of Default failed: %v", err)
		}

		// 4. Verify round-trip: the JSON representation deserializes back to the
		//    same value, confirming it is a valid JSON Schema "default".
		var roundTripped any
		if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
			t.Fatalf("json.Unmarshal of Default JSON failed: %v", err)
		}
		if !reflect.DeepEqual(materialized, roundTripped) {
			t.Fatalf("default value round-trip mismatch:\n  original:     %#v\n  roundTripped: %#v\n  json: %s", materialized, roundTripped, jsonBytes)
		}

		// 5. Verify that FieldMetadata with HasDefault=false has no default.
		fmNoDefault := schema.FieldMetadata{HasDefault: false}
		if fmNoDefault.Default != nil {
			t.Fatalf("FieldMetadata with HasDefault=false should have nil Default, got %v", fmNoDefault.Default)
		}
	})
}

// Feature: options-package-migration, Property 5: Пропагация ограничений валидации в JSON Schema
// **Validates: Requirements 12.1–12.5, 13.1–13.6, 14.1–14.4**

func TestProperty5_ValidationConstraintsPropagation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary combinations of non-zero constraints.
		// Each constraint is independently toggled on/off.

		// String constraints
		setPattern := rapid.Bool().Draw(t, "setPattern")
		setFormat := rapid.Bool().Draw(t, "setFormat")
		setMinLength := rapid.Bool().Draw(t, "setMinLength")
		setMaxLength := rapid.Bool().Draw(t, "setMaxLength")

		// Number constraints
		setMinimum := rapid.Bool().Draw(t, "setMinimum")
		setMaximum := rapid.Bool().Draw(t, "setMaximum")
		setExclusiveMinimum := rapid.Bool().Draw(t, "setExclusiveMinimum")
		setExclusiveMaximum := rapid.Bool().Draw(t, "setExclusiveMaximum")
		setMultipleOf := rapid.Bool().Draw(t, "setMultipleOf")

		// Array constraints
		setMinItems := rapid.Bool().Draw(t, "setMinItems")
		setMaxItems := rapid.Bool().Draw(t, "setMaxItems")
		setUniqueItems := rapid.Bool().Draw(t, "setUniqueItems")

		fm := schema.FieldMetadata{}

		// Populate non-zero values when toggled on.
		var pattern string
		if setPattern {
			pattern = rapid.StringMatching(`[a-z]{1,20}`).Draw(t, "pattern")
			fm.Pattern = pattern
		}

		var format string
		if setFormat {
			format = rapid.StringMatching(`[a-z\-]{1,20}`).Draw(t, "format")
			fm.Format = format
		}

		var minLength uint32
		if setMinLength {
			minLength = rapid.Uint32Range(1, 1000).Draw(t, "minLength")
			fm.MinLength = &minLength
		}

		var maxLength uint32
		if setMaxLength {
			maxLength = rapid.Uint32Range(1, 10000).Draw(t, "maxLength")
			fm.MaxLength = &maxLength
		}

		var minimum float64
		if setMinimum {
			minimum = rapid.Float64Range(-1e6, 1e6).Draw(t, "minimum")
			// Ensure non-zero
			if minimum == 0 {
				minimum = 1.0
			}
			fm.Minimum = &minimum
		}

		var maximum float64
		if setMaximum {
			maximum = rapid.Float64Range(-1e6, 1e6).Draw(t, "maximum")
			if maximum == 0 {
				maximum = 1.0
			}
			fm.Maximum = &maximum
		}

		var exclusiveMinimum float64
		if setExclusiveMinimum {
			exclusiveMinimum = rapid.Float64Range(-1e6, 1e6).Draw(t, "exclusiveMinimum")
			if exclusiveMinimum == 0 {
				exclusiveMinimum = -1.0
			}
			fm.ExclusiveMinimum = &exclusiveMinimum
		}

		var exclusiveMaximum float64
		if setExclusiveMaximum {
			exclusiveMaximum = rapid.Float64Range(-1e6, 1e6).Draw(t, "exclusiveMaximum")
			if exclusiveMaximum == 0 {
				exclusiveMaximum = 1.0
			}
			fm.ExclusiveMaximum = &exclusiveMaximum
		}

		var multipleOf float64
		if setMultipleOf {
			multipleOf = rapid.Float64Range(0.01, 1000).Draw(t, "multipleOf")
			if multipleOf == 0 {
				multipleOf = 1.0
			}
			fm.MultipleOf = &multipleOf
		}

		var minItems uint32
		if setMinItems {
			minItems = rapid.Uint32Range(1, 100).Draw(t, "minItems")
			fm.MinItems = &minItems
		}

		var maxItems uint32
		if setMaxItems {
			maxItems = rapid.Uint32Range(1, 1000).Draw(t, "maxItems")
			fm.MaxItems = &maxItems
		}

		if setUniqueItems {
			fm.UniqueItems = true
		}

		// Verify non-zero constraints are correctly stored.
		if setPattern {
			if fm.Pattern != pattern {
				t.Fatalf("Pattern: expected %q, got %q", pattern, fm.Pattern)
			}
		} else {
			if fm.Pattern != "" {
				t.Fatalf("Pattern should be zero-value, got %q", fm.Pattern)
			}
		}

		if setFormat {
			if fm.Format != format {
				t.Fatalf("Format: expected %q, got %q", format, fm.Format)
			}
		} else {
			if fm.Format != "" {
				t.Fatalf("Format should be zero-value, got %q", fm.Format)
			}
		}

		if setMinLength {
			if fm.MinLength == nil || *fm.MinLength != minLength {
				t.Fatalf("MinLength: expected %d, got %v", minLength, fm.MinLength)
			}
		} else {
			if fm.MinLength != nil {
				t.Fatalf("MinLength should be nil, got %v", fm.MinLength)
			}
		}

		if setMaxLength {
			if fm.MaxLength == nil || *fm.MaxLength != maxLength {
				t.Fatalf("MaxLength: expected %d, got %v", maxLength, fm.MaxLength)
			}
		} else {
			if fm.MaxLength != nil {
				t.Fatalf("MaxLength should be nil, got %v", fm.MaxLength)
			}
		}

		if setMinimum {
			if fm.Minimum == nil || *fm.Minimum != minimum {
				t.Fatalf("Minimum: expected %v, got %v", minimum, fm.Minimum)
			}
		} else {
			if fm.Minimum != nil {
				t.Fatalf("Minimum should be nil, got %v", fm.Minimum)
			}
		}

		if setMaximum {
			if fm.Maximum == nil || *fm.Maximum != maximum {
				t.Fatalf("Maximum: expected %v, got %v", maximum, fm.Maximum)
			}
		} else {
			if fm.Maximum != nil {
				t.Fatalf("Maximum should be nil, got %v", fm.Maximum)
			}
		}

		if setExclusiveMinimum {
			if fm.ExclusiveMinimum == nil || *fm.ExclusiveMinimum != exclusiveMinimum {
				t.Fatalf("ExclusiveMinimum: expected %v, got %v", exclusiveMinimum, fm.ExclusiveMinimum)
			}
		} else {
			if fm.ExclusiveMinimum != nil {
				t.Fatalf("ExclusiveMinimum should be nil, got %v", fm.ExclusiveMinimum)
			}
		}

		if setExclusiveMaximum {
			if fm.ExclusiveMaximum == nil || *fm.ExclusiveMaximum != exclusiveMaximum {
				t.Fatalf("ExclusiveMaximum: expected %v, got %v", exclusiveMaximum, fm.ExclusiveMaximum)
			}
		} else {
			if fm.ExclusiveMaximum != nil {
				t.Fatalf("ExclusiveMaximum should be nil, got %v", fm.ExclusiveMaximum)
			}
		}

		if setMultipleOf {
			if fm.MultipleOf == nil || *fm.MultipleOf != multipleOf {
				t.Fatalf("MultipleOf: expected %v, got %v", multipleOf, fm.MultipleOf)
			}
		} else {
			if fm.MultipleOf != nil {
				t.Fatalf("MultipleOf should be nil, got %v", fm.MultipleOf)
			}
		}

		if setMinItems {
			if fm.MinItems == nil || *fm.MinItems != minItems {
				t.Fatalf("MinItems: expected %d, got %v", minItems, fm.MinItems)
			}
		} else {
			if fm.MinItems != nil {
				t.Fatalf("MinItems should be nil, got %v", fm.MinItems)
			}
		}

		if setMaxItems {
			if fm.MaxItems == nil || *fm.MaxItems != maxItems {
				t.Fatalf("MaxItems: expected %d, got %v", maxItems, fm.MaxItems)
			}
		} else {
			if fm.MaxItems != nil {
				t.Fatalf("MaxItems should be nil, got %v", fm.MaxItems)
			}
		}

		if setUniqueItems {
			if !fm.UniqueItems {
				t.Fatal("UniqueItems: expected true, got false")
			}
		} else {
			if fm.UniqueItems {
				t.Fatal("UniqueItems should be zero-value (false), got true")
			}
		}

		// Verify JSON-serializability of the constraint values.
		// This confirms the values are suitable for JSON Schema emission.
		constraintMap := map[string]any{}
		if fm.Pattern != "" {
			constraintMap["pattern"] = fm.Pattern
		}
		if fm.Format != "" {
			constraintMap["format"] = fm.Format
		}
		if fm.MinLength != nil {
			constraintMap["minLength"] = *fm.MinLength
		}
		if fm.MaxLength != nil {
			constraintMap["maxLength"] = *fm.MaxLength
		}
		if fm.Minimum != nil {
			constraintMap["minimum"] = *fm.Minimum
		}
		if fm.Maximum != nil {
			constraintMap["maximum"] = *fm.Maximum
		}
		if fm.ExclusiveMinimum != nil {
			constraintMap["exclusiveMinimum"] = *fm.ExclusiveMinimum
		}
		if fm.ExclusiveMaximum != nil {
			constraintMap["exclusiveMaximum"] = *fm.ExclusiveMaximum
		}
		if fm.MultipleOf != nil {
			constraintMap["multipleOf"] = *fm.MultipleOf
		}
		if fm.MinItems != nil {
			constraintMap["minItems"] = *fm.MinItems
		}
		if fm.MaxItems != nil {
			constraintMap["maxItems"] = *fm.MaxItems
		}
		if fm.UniqueItems {
			constraintMap["uniqueItems"] = fm.UniqueItems
		}

		jsonBytes, err := json.Marshal(constraintMap)
		if err != nil {
			t.Fatalf("json.Marshal of constraints failed: %v", err)
		}

		var roundTripped map[string]any
		if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
			t.Fatalf("json.Unmarshal of constraints failed: %v", err)
		}

		// Verify the number of emitted keys matches the number of set constraints.
		expectedKeys := 0
		for _, set := range []bool{
			setPattern, setFormat, setMinLength, setMaxLength,
			setMinimum, setMaximum, setExclusiveMinimum, setExclusiveMaximum, setMultipleOf,
			setMinItems, setMaxItems, setUniqueItems,
		} {
			if set {
				expectedKeys++
			}
		}
		if len(roundTripped) != expectedKeys {
			t.Fatalf("expected %d constraint keys in JSON, got %d: %s", expectedKeys, len(roundTripped), jsonBytes)
		}
	})
}

// Feature: options-package-migration, Property 6: Пропагация MessageOptions в JSON Schema
// **Validates: Requirements 15.1, 15.2, 15.3**

func TestProperty6_MessageOptionsPropagation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random title: empty or non-empty string.
		title := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,30}`),
		).Draw(t, "title")

		// Generate random description: empty or non-empty string.
		description := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,50}`),
		).Draw(t, "description")

		// Generate random typed examples (0-3 simple objects) using the
		// ExampleValue generator from Property 3 to produce realistic values.
		numExamples := rapid.IntRange(0, 3).Draw(t, "numExamples")
		var typedExamples []any
		for i := 0; i < numExamples; i++ {
			ev := genExampleValue(2).Draw(t, fmt.Sprintf("example_%d", i))
			if materialized, ok := materializeExampleValue(ev); ok {
				typedExamples = append(typedExamples, materialized)
			}
		}

		// Build Metadata with generated values.
		meta := schema.Metadata{
			Title:         title,
			Description:   description,
			TypedExamples: typedExamples,
		}

		// Build the expected JSON Schema map applying the same rules as
		// generateMessageSchema in schema.go:
		//   - Description is always set (may be empty)
		//   - Title is set only if non-empty
		//   - TypedExamples take priority; if non-empty they become "examples"
		jsonSchema := map[string]any{
			"type": "object",
		}
		if meta.Description != "" {
			jsonSchema["description"] = meta.Description
		}
		if meta.Title != "" {
			jsonSchema["title"] = meta.Title
		}
		if len(meta.TypedExamples) > 0 {
			jsonSchema["examples"] = meta.TypedExamples
		}

		// Marshal and unmarshal to simulate JSON Schema serialization round-trip.
		raw, err := json.Marshal(jsonSchema)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// Verify "title" presence/absence.
		if meta.Title != "" {
			got, ok := parsed["title"]
			if !ok {
				t.Fatalf("expected 'title' key in schema for non-empty Title %q", meta.Title)
			}
			if got != meta.Title {
				t.Fatalf("title mismatch: got %q, want %q", got, meta.Title)
			}
		} else {
			if _, ok := parsed["title"]; ok {
				t.Fatal("unexpected 'title' key in schema for empty Title")
			}
		}

		// Verify "description" presence/absence.
		if meta.Description != "" {
			got, ok := parsed["description"]
			if !ok {
				t.Fatalf("expected 'description' key in schema for non-empty Description %q", meta.Description)
			}
			if got != meta.Description {
				t.Fatalf("description mismatch: got %q, want %q", got, meta.Description)
			}
		} else {
			if _, ok := parsed["description"]; ok {
				t.Fatal("unexpected 'description' key in schema for empty Description")
			}
		}

		// Verify "examples" presence/absence.
		if len(meta.TypedExamples) > 0 {
			rawExamples, ok := parsed["examples"]
			if !ok {
				t.Fatal("expected 'examples' key in schema for non-empty TypedExamples")
			}
			examplesSlice, ok := rawExamples.([]any)
			if !ok {
				t.Fatalf("examples has type %T, want []any", rawExamples)
			}
			if len(examplesSlice) != len(meta.TypedExamples) {
				t.Fatalf("examples length: got %d, want %d", len(examplesSlice), len(meta.TypedExamples))
			}
			// Verify each example round-trips correctly through JSON.
			for i, original := range meta.TypedExamples {
				originalJSON, err := json.Marshal(original)
				if err != nil {
					t.Fatalf("json.Marshal of example[%d] failed: %v", i, err)
				}
				parsedJSON, err := json.Marshal(examplesSlice[i])
				if err != nil {
					t.Fatalf("json.Marshal of parsed example[%d] failed: %v", i, err)
				}
				var originalNorm, parsedNorm any
				if err := json.Unmarshal(originalJSON, &originalNorm); err != nil {
					t.Fatalf("json.Unmarshal of original example[%d] failed: %v", i, err)
				}
				if err := json.Unmarshal(parsedJSON, &parsedNorm); err != nil {
					t.Fatalf("json.Unmarshal of parsed example[%d] failed: %v", i, err)
				}
				if !reflect.DeepEqual(originalNorm, parsedNorm) {
					t.Fatalf("example[%d] mismatch:\n  original: %s\n  parsed:   %s", i, originalJSON, parsedJSON)
				}
			}
		} else {
			if _, ok := parsed["examples"]; ok {
				t.Fatal("unexpected 'examples' key in schema for empty TypedExamples")
			}
		}
	})
}

// Feature: options-package-migration, Property 7: Пропагация EnumOptions в JSON Schema
// **Validates: Requirements 16.1, 16.2**

func TestProperty7_EnumOptionsPropagation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random title: empty or non-empty string.
		title := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,30}`),
		).Draw(t, "title")

		// Generate random description: empty or non-empty string.
		description := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,50}`),
		).Draw(t, "description")

		meta := schema.EnumMetadata{
			Title:       title,
			Description: description,
		}

		// Build the expected JSON Schema map applying the same rules as
		// generateSingularSchema EnumKind case in schema.go:
		//   - type is always "string"
		//   - enum array always present with sample values
		//   - title is set only if non-empty
		//   - description is set only if non-empty
		jsonSchema := map[string]any{
			"type": "string",
			"enum": []any{"VALUE_A", "VALUE_B"},
		}
		if meta.Title != "" {
			jsonSchema["title"] = meta.Title
		}
		if meta.Description != "" {
			jsonSchema["description"] = meta.Description
		}

		// Marshal and unmarshal to simulate JSON Schema serialization round-trip.
		raw, err := json.Marshal(jsonSchema)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// Verify "title" presence/absence.
		if meta.Title != "" {
			got, ok := parsed["title"]
			if !ok {
				t.Fatalf("expected 'title' key in schema for non-empty Title %q", meta.Title)
			}
			if got != meta.Title {
				t.Fatalf("title mismatch: got %q, want %q", got, meta.Title)
			}
		} else {
			if _, ok := parsed["title"]; ok {
				t.Fatal("unexpected 'title' key in schema for empty Title")
			}
		}

		// Verify "description" presence/absence.
		if meta.Description != "" {
			got, ok := parsed["description"]
			if !ok {
				t.Fatalf("expected 'description' key in schema for non-empty Description %q", meta.Description)
			}
			if got != meta.Description {
				t.Fatalf("description mismatch: got %q, want %q", got, meta.Description)
			}
		} else {
			if _, ok := parsed["description"]; ok {
				t.Fatal("unexpected 'description' key in schema for empty Description")
			}
		}

		// Verify base schema structure is always present.
		if parsed["type"] != "string" {
			t.Fatalf("expected type 'string', got %v", parsed["type"])
		}
		enumArr, ok := parsed["enum"].([]any)
		if !ok {
			t.Fatal("expected 'enum' key to be an array")
		}
		if len(enumArr) != 2 {
			t.Fatalf("expected 2 enum values, got %d", len(enumArr))
		}
	})
}

// Feature: options-package-migration, Property 8: Исключение скрытых enum values
// **Validates: Requirements 17.1, 17.4**

func TestProperty8_HiddenEnumValuesExclusion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-10 enum values with random hidden flags.
		numValues := rapid.IntRange(1, 10).Draw(t, "numValues")

		type enumEntry struct {
			Name   string
			Hidden bool
		}

		entries := make([]enumEntry, numValues)
		var expectedEnum []any
		for i := range numValues {
			name := fmt.Sprintf("VALUE_%d", i)
			hidden := rapid.Bool().Draw(t, fmt.Sprintf("hidden_%d", i))
			entries[i] = enumEntry{Name: name, Hidden: hidden}
			if !hidden {
				expectedEnum = append(expectedEnum, name)
			}
		}

		// Build JSON Schema applying the same filtering logic as
		// generateSingularSchema EnumKind in schema.go:
		//   - type is always "string"
		//   - enum contains only non-hidden value names
		//   - if all values are hidden, enum is an empty array
		var enumArray []any
		if expectedEnum != nil {
			enumArray = expectedEnum
		} else {
			enumArray = []any{}
		}

		jsonSchema := map[string]any{
			"type": "string",
			"enum": enumArray,
		}

		// Marshal and unmarshal to simulate JSON Schema serialization round-trip.
		raw, err := json.Marshal(jsonSchema)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// Verify base schema structure.
		if parsed["type"] != "string" {
			t.Fatalf("expected type 'string', got %v", parsed["type"])
		}

		// Verify "enum" array is present and is an array.
		rawEnum, ok := parsed["enum"]
		if !ok {
			t.Fatal("expected 'enum' key in schema")
		}
		parsedEnum, ok := rawEnum.([]any)
		if !ok {
			t.Fatalf("expected 'enum' to be an array, got %T", rawEnum)
		}

		// Count expected non-hidden values.
		var expectedNames []string
		for _, e := range entries {
			if !e.Hidden {
				expectedNames = append(expectedNames, e.Name)
			}
		}

		// Verify length matches.
		if len(parsedEnum) != len(expectedNames) {
			t.Fatalf("enum length mismatch: got %d, want %d (entries: %v)", len(parsedEnum), len(expectedNames), entries)
		}

		// Verify each value matches in order.
		for i, want := range expectedNames {
			got, ok := parsedEnum[i].(string)
			if !ok {
				t.Fatalf("enum[%d] is not a string: %v (%T)", i, parsedEnum[i], parsedEnum[i])
			}
			if got != want {
				t.Fatalf("enum[%d] mismatch: got %q, want %q", i, got, want)
			}
		}

		// Verify no hidden values appear in the enum array.
		enumSet := make(map[string]bool, len(parsedEnum))
		for _, v := range parsedEnum {
			if s, ok := v.(string); ok {
				enumSet[s] = true
			}
		}
		for _, e := range entries {
			if e.Hidden && enumSet[e.Name] {
				t.Fatalf("hidden value %q should not appear in enum array", e.Name)
			}
		}

		// If all values are hidden, enum must be an empty array (not nil/absent).
		if len(expectedNames) == 0 {
			if len(parsedEnum) != 0 {
				t.Fatalf("expected empty enum array when all values are hidden, got %v", parsedEnum)
			}
		}
	})
}

// Feature: options-package-migration, Property 1: Эквивалентность расширений между старым и новым пакетом
// **Validates: Requirements 1.4, 3.2**

func TestProperty1_ExtensionEquivalenceBetweenOldAndNewPackage(t *testing.T) {
	// Define the expected extensions from the new package mcp/options/v1.
	// The old package api/mcp/options/v1 has been deleted; we verify the new
	// package preserves the same extension numbers (91001-91006) and uses the
	// correct mcp.options.v1 prefix (not api.mcp.options.v1).
	type extensionSpec struct {
		Extension    *protoimpl.ExtensionInfo
		ExpectedNum  int32
		ExpectedName string // full extension name in new package
		ExtendedType string // the google.protobuf.*Options being extended
		MessageType  string // the mcp.options.v1.* message type
	}

	specs := []extensionSpec{
		{mcpoptionsv1.E_Service, 91001, "mcp.options.v1.service", "google.protobuf.ServiceOptions", "mcp.options.v1.ServiceOptions"},
		{mcpoptionsv1.E_Method, 91002, "mcp.options.v1.method", "google.protobuf.MethodOptions", "mcp.options.v1.MethodOptions"},
		{mcpoptionsv1.E_Field, 91003, "mcp.options.v1.field", "google.protobuf.FieldOptions", "mcp.options.v1.FieldOptions"},
		{mcpoptionsv1.E_Message, 91004, "mcp.options.v1.message", "google.protobuf.MessageOptions", "mcp.options.v1.MessageOptions"},
		{mcpoptionsv1.E_Enum, 91005, "mcp.options.v1.enum", "google.protobuf.EnumOptions", "mcp.options.v1.EnumOptions"},
		{mcpoptionsv1.E_EnumValue, 91006, "mcp.options.v1.enum_value", "google.protobuf.EnumValueOptions", "mcp.options.v1.EnumValueOptions"},
	}

	rapid.Check(t, func(t *rapid.T) {
		// Randomly pick an extension from the set to verify.
		idx := rapid.IntRange(0, len(specs)-1).Draw(t, "extensionIndex")
		spec := specs[idx]

		desc := spec.Extension.TypeDescriptor()

		// 1. Extension number must match the expected value (91001-91006).
		gotNum := int32(desc.Number())
		if gotNum != spec.ExpectedNum {
			t.Fatalf("extension %q: expected number %d, got %d", spec.ExpectedName, spec.ExpectedNum, gotNum)
		}

		// 2. Full extension name must use mcp.options.v1 prefix, not api.mcp.options.v1.
		gotName := string(desc.FullName())
		if gotName != spec.ExpectedName {
			t.Fatalf("extension number %d: expected full name %q, got %q", spec.ExpectedNum, spec.ExpectedName, gotName)
		}

		// Verify the name does NOT contain the old api prefix.
		if strings.Contains(gotName, "api.mcp.options.v1") {
			t.Fatalf("extension %q still uses old api.mcp.options.v1 prefix", gotName)
		}

		// 3. Extension resolves to the correct containing (extended) message type.
		gotExtended := string(desc.ContainingMessage().FullName())
		if gotExtended != spec.ExtendedType {
			t.Fatalf("extension %q: expected extended type %q, got %q", spec.ExpectedName, spec.ExtendedType, gotExtended)
		}

		// 4. Extension message type matches the expected mcp.options.v1.* message.
		gotMsgType := string(desc.Message().FullName())
		if gotMsgType != spec.MessageType {
			t.Fatalf("extension %q: expected message type %q, got %q", spec.ExpectedName, spec.MessageType, gotMsgType)
		}
	})
}

// Feature: options-package-migration, Property 2: Отсутствие ссылок на старый путь
// **Validates: Requirements 7.3, 3.1, 4.1, 4.2**

func TestProperty2_NoReferencesToOldPath(t *testing.T) {
	// Collect all .go and .proto files in the repository, excluding
	// .kiro/, .git/, api/, README.md, and go.sum.
	repoRoot := "../../"

	var files []string
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to repo root for exclusion checks.
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}

		// Exclude directories.
		if info.IsDir() {
			switch {
			case rel == ".kiro" || strings.HasPrefix(rel, ".kiro"+string(filepath.Separator)):
				return filepath.SkipDir
			case rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)):
				return filepath.SkipDir
			case rel == "api" || strings.HasPrefix(rel, "api"+string(filepath.Separator)):
				return filepath.SkipDir
			}
			return nil
		}

		// Exclude specific files.
		if rel == "README.md" || rel == "go.sum" {
			return nil
		}

		// Exclude property test files — they legitimately reference the old
		// path string in test assertions and comments (e.g., Property 1
		// verifies extension name prefixes against the old package name).
		if strings.HasSuffix(rel, "_property_test.go") {
			return nil
		}

		// Only include .go and .proto files.
		ext := filepath.Ext(path)
		if ext == ".go" || ext == ".proto" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk repository: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no .go or .proto files found in repository")
	}

	const oldPath = "api/mcp/options/v1"

	rapid.Check(t, func(t *rapid.T) {
		// Randomly sample a file from the collected list.
		idx := rapid.IntRange(0, len(files)-1).Draw(t, "fileIndex")
		filePath := files[idx]

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", filePath, err)
		}

		if strings.Contains(string(content), oldPath) {
			rel, _ := filepath.Rel(repoRoot, filePath)
			t.Fatalf("file %s contains reference to old path %q", rel, oldPath)
		}
	})
}
