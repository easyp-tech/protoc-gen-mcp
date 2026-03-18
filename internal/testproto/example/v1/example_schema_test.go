package examplev1

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type schemaDocument struct {
	Ref                  string                    `json:"$ref,omitempty"`
	Defs                 map[string]schemaDocument `json:"$defs,omitempty"`
	Type                 any                       `json:"type,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Properties           map[string]schemaDocument `json:"properties,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	Enum                 []string                  `json:"enum,omitempty"`
	Examples             []any                     `json:"examples,omitempty"`
	Items                any                       `json:"items,omitempty"`
	AnyOf                []schemaDocument          `json:"anyOf,omitempty"`
	AllOf                []schemaDocument          `json:"allOf,omitempty"`
	OneOf                []schemaDocument          `json:"oneOf,omitempty"`
	AdditionalProperties any                       `json:"additionalProperties,omitempty"`
	PropertyNames        *schemaDocument           `json:"propertyNames,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Pattern              string                    `json:"pattern,omitempty"`
}

func TestGeneratedSchemasMatchProtoJSONContract(t *testing.T) {
	t.Run("create_report_input", func(t *testing.T) {
		schema := decodeSchema(t, ExampleAPI_CreateReport_ToolSpecInputSchemaJSON)

		if got := schemaTypeString(t, schema.Type); got != "object" {
			t.Fatalf("input schema type = %q, want object", got)
		}
		if !slices.Equal(schema.Required, []string{"city", "count", "details"}) {
			t.Fatalf("input required = %v, want [city count details]", schema.Required)
		}
		if len(schema.Examples) != 1 || exampleString(t, schema.Examples[0]) != `{"city":"Paris","count":2,"details":{"label":"today"}}` {
			t.Fatalf("input examples = %v, want fixture example", schema.Examples)
		}

		details := schema.Properties["details"]
		if got := schemaTypeString(t, details.Type); got != "object" {
			t.Fatalf("details.type = %q, want object", got)
		}
		if !slices.Equal(details.Required, []string{"label"}) {
			t.Fatalf("details.required = %v, want [label]", details.Required)
		}

		labels := schema.Properties["labels"]
		if got := schemaTypeList(t, labels.Type); !slices.Equal(got, []string{"array", "null"}) {
			t.Fatalf("labels.types = %v, want [array null]", got)
		}
		if labels.Items == nil || schemaTypeString(t, decodeNestedSchema(t, labels.Items).Type) != "string" {
			t.Fatalf("labels.items.type = %v, want string", labels.Items)
		}
		labelItems := decodeNestedSchema(t, labels.Items)
		if len(labelItems.Examples) != 1 || exampleString(t, labelItems.Examples[0]) != "example" {
			t.Fatalf("labels.items.examples = %v, want [example]", labelItems.Examples)
		}
		if len(labels.Examples) != 1 || !slices.Equal(exampleStringSlice(t, labels.Examples[0]), []string{"example"}) {
			t.Fatalf("labels.examples = %v, want [[example]]", labels.Examples)
		}

		if _, found := schema.Properties["units"]; !found {
			t.Fatal("units property missing from input schema")
		}
		if slices.Contains(schema.Required, "units") {
			t.Fatal("units unexpectedly marked required")
		}
		if got := schemaTypeList(t, schema.Properties["units"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("units.types = %v, want [string null]", got)
		}
	})

	t.Run("create_report_output", func(t *testing.T) {
		schema := decodeSchema(t, ExampleAPI_CreateReport_ToolSpecOutputSchemaJSON)

		if !slices.Equal(schema.Required, []string{"reportId", "totalCount", "status", "details"}) {
			t.Fatalf("output required = %v, want [reportId totalCount status details]", schema.Required)
		}

		totalCount := schema.Properties["totalCount"]
		if got := schemaTypeString(t, totalCount.Type); got != "string" {
			t.Fatalf("totalCount.type = %q, want string for ProtoJSON int64", got)
		}

		status := schema.Properties["status"]
		if !slices.Equal(status.Enum, []string{"REPORT_STATUS_OK", "REPORT_STATUS_FAILED"}) {
			t.Fatalf("status.enum = %v, want enum names without hidden NONE", status.Enum)
		}
	})

	t.Run("ping_output_empty_support", func(t *testing.T) {
		schema := decodeSchema(t, ExampleAPI_Ping_ToolSpecOutputSchemaJSON)

		if !slices.Equal(schema.Required, []string{"ack"}) {
			t.Fatalf("ping output required = %v, want [ack]", schema.Required)
		}

		ack := schema.Properties["ack"]
		if got := schemaTypeString(t, ack.Type); got != "object" {
			t.Fatalf("ack.type = %q, want object", got)
		}

		additionalProperties, ok := ack.AdditionalProperties.(bool)
		if !ok {
			t.Fatalf("ack.additionalProperties has type %T, want bool", ack.AdditionalProperties)
		}
		if additionalProperties {
			t.Fatal("ack.additionalProperties = true, want false")
		}
	})

	t.Run("advanced_shapes_support", func(t *testing.T) {
		schema := decodeSchema(t, ExampleAPI_DescribeAdvancedShapes_ToolSpecInputSchemaJSON)

		labels := schema.Properties["labels"]
		if got := schemaTypeList(t, labels.Type); !slices.Equal(got, []string{"object", "null"}) {
			t.Fatalf("labels.types = %v, want [object null]", got)
		}
		if len(labels.Examples) != 1 {
			t.Fatalf("labels.examples length = %d, want 1", len(labels.Examples))
		}
		if got := exampleObject(t, labels.Examples[0])["key"]; got != "example" {
			t.Fatalf("labels.examples[0].key = %v, want example", got)
		}

		labelValues := decodeNestedSchema(t, labels.AdditionalProperties)
		if got := schemaTypeString(t, labelValues.Type); got != "string" {
			t.Fatalf("labels.additionalProperties.type = %q, want string", got)
		}

		quantities := schema.Properties["quantities"]
		if quantities.PropertyNames == nil || quantities.PropertyNames.Pattern == "" {
			t.Fatal("quantities.propertyNames.pattern is empty, want numeric key validation")
		}
		if got := exampleObject(t, quantities.Examples[0])["1"]; got != "example" {
			t.Fatalf("quantities.examples[0][1] = %v, want example", got)
		}

		limits := schema.Properties["limits"]
		if limits.PropertyNames == nil || limits.PropertyNames.Pattern == "" {
			t.Fatal("limits.propertyNames.pattern is empty, want unsigned numeric key validation")
		}
		if got := exampleObject(t, limits.Examples[0])["1"]; got != "example" {
			t.Fatalf("limits.examples[0][1] = %v, want example", got)
		}

		toggles := schema.Properties["toggles"]
		if toggles.PropertyNames == nil || !slices.Equal(toggles.PropertyNames.Enum, []string{"true", "false"}) {
			t.Fatalf("toggles.propertyNames.enum = %v, want [true false]", toggles.PropertyNames)
		}
		if got := exampleObject(t, toggles.Examples[0])["true"]; got != "example" {
			t.Fatalf("toggles.examples[0][true] = %v, want example", got)
		}

		observedAt := schema.Properties["observedAt"]
		if got := schemaTypeList(t, observedAt.Type); !slices.Equal(got, []string{"string", "null"}) || observedAt.Format != "date-time" {
			t.Fatalf("observedAt schema = %+v, want string/date-time", observedAt)
		}

		ttl := schema.Properties["ttl"]
		if got := schemaTypeList(t, ttl.Type); !slices.Equal(got, []string{"string", "null"}) || ttl.Pattern == "" {
			t.Fatalf("ttl schema = %+v, want string with duration pattern", ttl)
		}

		note := schema.Properties["note"]
		if got := schemaTypeList(t, note.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("note.types = %v, want [string null]", got)
		}

		total := schema.Properties["total"]
		if got := schemaTypeList(t, total.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("total.types = %v, want [string null]", got)
		}

		blob := schema.Properties["blob"]
		if got := schemaTypeList(t, blob.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("blob.types = %v, want [string null]", got)
		}

		detailAny := schema.Properties["detailAny"]
		if got := schemaTypeList(t, detailAny.Type); !slices.Equal(got, []string{"object", "null"}) {
			t.Fatalf("detailAny.types = %v, want [object null]", got)
		}
		if !slices.Equal(detailAny.Required, []string{"@type"}) {
			t.Fatalf("detailAny.required = %v, want [@type]", detailAny.Required)
		}
		if detailAny.Description == "" || !containsAll(detailAny.Description, "detail_any", "@type", "value") {
			t.Fatalf("detailAny.description = %q, want field and Any guidance", detailAny.Description)
		}
		detailAnyExample := exampleObject(t, detailAny.Examples[0])
		if detailAnyExample["@type"] != "type.googleapis.com/internal.testproto.example.v1.ReportDetails" {
			t.Fatalf("detailAny.examples[0].@type = %v, want ReportDetails type URL", detailAnyExample["@type"])
		}
		if detailAnyExample["label"] != "from-any" {
			t.Fatalf("detailAny.examples[0].label = %v, want from-any", detailAnyExample["label"])
		}
		if got := schemaTypeString(t, detailAny.Properties["@type"].Type); got != "string" {
			t.Fatalf("detailAny.@type.type = %q, want string", got)
		}
		if detailAny.Properties["@type"].Description == "" || !containsAll(detailAny.Properties["@type"].Description, "type.googleapis.com", "message") {
			t.Fatalf("detailAny.@type.description = %q, want type URL guidance", detailAny.Properties["@type"].Description)
		}
		if additionalProperties, ok := detailAny.AdditionalProperties.(bool); !ok || !additionalProperties {
			t.Fatalf("detailAny.additionalProperties = %#v, want true", detailAny.AdditionalProperties)
		}

		durationAny := schema.Properties["durationAny"]
		if got := schemaTypeList(t, durationAny.Type); !slices.Equal(got, []string{"object", "null"}) {
			t.Fatalf("durationAny.types = %v, want [object null]", got)
		}
		if !slices.Equal(durationAny.Required, []string{"@type"}) {
			t.Fatalf("durationAny.required = %v, want [@type]", durationAny.Required)
		}
		durationAnyExample := exampleObject(t, durationAny.Examples[0])
		if durationAnyExample["@type"] != "type.googleapis.com/google.protobuf.Duration" {
			t.Fatalf("durationAny.examples[0].@type = %v, want Duration type URL", durationAnyExample["@type"])
		}
		if durationAnyExample["value"] != "3600s" {
			t.Fatalf("durationAny.examples[0].value = %v, want 3600s", durationAnyExample["value"])
		}

		cityAlias := schema.Properties["cityAlias"]
		if got := schemaTypeList(t, cityAlias.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("cityAlias.types = %v, want [string null]", got)
		}

		cityID := schema.Properties["cityId"]
		if got := schemaTypeList(t, cityID.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("cityId.types = %v, want [string null]", got)
		}

		cityDetails := schema.Properties["cityDetails"]
		if got := schemaTypeList(t, cityDetails.Type); !slices.Equal(got, []string{"object", "null"}) {
			t.Fatalf("cityDetails.types = %v, want [object null]", got)
		}

		smallTotal := schema.Properties["smallTotal"]
		if got := schemaTypeList(t, smallTotal.Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("smallTotal.types = %v, want [integer null]", got)
		}

		uintTotal := schema.Properties["uintTotal"]
		if got := schemaTypeList(t, uintTotal.Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("uintTotal.types = %v, want [integer null]", got)
		}

		hugeTotal := schema.Properties["hugeTotal"]
		if got := schemaTypeList(t, hugeTotal.Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("hugeTotal.types = %v, want [string null]", got)
		}

		weight := schema.Properties["weight"]
		if len(weight.AnyOf) != 3 {
			t.Fatalf("weight.anyOf length = %d, want 3", len(weight.AnyOf))
		}

		ratio := schema.Properties["ratio"]
		if len(ratio.AnyOf) != 3 {
			t.Fatalf("ratio.anyOf length = %d, want 3", len(ratio.AnyOf))
		}
		if got := schemaTypeString(t, ratio.AnyOf[0].Type); got != "number" {
			t.Fatalf("ratio.anyOf[0].type = %q, want number", got)
		}
		if !slices.Equal(ratio.AnyOf[1].Enum, []string{"NaN", "Infinity", "-Infinity"}) {
			t.Fatalf("ratio.anyOf[1].enum = %v, want special float strings", ratio.AnyOf[1].Enum)
		}
		if got := schemaTypeString(t, ratio.AnyOf[2].Type); got != "null" {
			t.Fatalf("ratio.anyOf[2].type = %q, want null", got)
		}

		rawRatio := schema.Properties["rawRatio"]
		if len(rawRatio.AnyOf) != 3 {
			t.Fatalf("rawRatio.anyOf length = %d, want 3", len(rawRatio.AnyOf))
		}
		if got := schemaTypeString(t, rawRatio.AnyOf[2].Type); got != "null" {
			t.Fatalf("rawRatio.anyOf[2].type = %q, want null", got)
		}

		if len(schema.AllOf) == 0 {
			t.Fatal("advanced schema missing allOf oneof constraints")
		}
		if len(schema.AllOf[0].OneOf) != 4 {
			t.Fatalf("advanced schema oneof branch count = %d, want 4", len(schema.AllOf[0].OneOf))
		}

		tree := schema.Properties["tree"]
		if got := schemaTypeList(t, tree.Type); !slices.Equal(got, []string{"object", "null"}) {
			t.Fatalf("tree.types = %v, want [object null]", got)
		}
		child := tree.Properties["child"]
		if len(child.AnyOf) != 2 {
			t.Fatalf("tree.child.anyOf length = %d, want 2", len(child.AnyOf))
		}
		if child.AnyOf[0].Ref != "#/$defs/internal.testproto.example.v1.RecursiveNode" {
			t.Fatalf("tree.child.anyOf[0].$ref = %q, want recursive definition ref", child.AnyOf[0].Ref)
		}
		if got := schemaTypeString(t, child.AnyOf[1].Type); got != "null" {
			t.Fatalf("tree.child.anyOf[1].type = %q, want null", got)
		}
		childrenField := tree.Properties["children"]
		if got := schemaTypeList(t, childrenField.Type); !slices.Equal(got, []string{"array", "null"}) {
			t.Fatalf("tree.children.types = %v, want [array null]", got)
		}
		children := decodeNestedSchema(t, childrenField.Items)
		if children.Ref != "#/$defs/internal.testproto.example.v1.RecursiveNode" {
			t.Fatalf("tree.children.items.$ref = %q, want recursive definition ref", children.Ref)
		}
		if len(children.Examples) != 1 {
			t.Fatalf("tree.children.items.examples length = %d, want 1", len(children.Examples))
		}
		if exampleObject(t, children.Examples[0])["name"] != "example" {
			t.Fatalf("tree.children.items.examples[0].name = %v, want example", exampleObject(t, children.Examples[0])["name"])
		}

		recursiveNode, ok := schema.Defs["internal.testproto.example.v1.RecursiveNode"]
		if !ok {
			t.Fatal("schema missing recursive node definition")
		}
		if got := schemaTypeString(t, recursiveNode.Type); got != "object" {
			t.Fatalf("recursive node definition type = %q, want object", got)
		}
		if !slices.Equal(recursiveNode.Required, []string{"name"}) {
			t.Fatalf("recursive node required = %v, want [name]", recursiveNode.Required)
		}
	})

	t.Run("scalar_shapes_support", func(t *testing.T) {
		schema := decodeSchema(t, ExampleAPI_DescribeScalarShapes_ToolSpecInputSchemaJSON)

		if got := schemaTypeString(t, schema.Type); got != "object" {
			t.Fatalf("scalar schema type = %q, want object", got)
		}

		requiredFields := []string{
			"boolFlag",
			"textValue",
			"bytesValue",
			"int32Value",
			"sint32Value",
			"sfixed32Value",
			"uint32Value",
			"fixed32Value",
			"int64Value",
			"sint64Value",
			"sfixed64Value",
			"uint64Value",
			"fixed64Value",
			"floatValue",
			"doubleValue",
			"status",
			"details",
		}
		for _, name := range requiredFields {
			if !slices.Contains(schema.Required, name) {
				t.Fatalf("scalar schema missing required field %q in %v", name, schema.Required)
			}
		}
		if slices.Contains(schema.Required, "optionalInt32Value") {
			t.Fatalf("optionalInt32Value unexpectedly required in %v", schema.Required)
		}

		if got := schemaTypeString(t, schema.Properties["boolFlag"].Type); got != "boolean" {
			t.Fatalf("boolFlag.type = %q, want boolean", got)
		}
		if got := schemaTypeString(t, schema.Properties["textValue"].Type); got != "string" {
			t.Fatalf("textValue.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["bytesValue"].Type); got != "string" {
			t.Fatalf("bytesValue.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["int32Value"].Type); got != "integer" {
			t.Fatalf("int32Value.type = %q, want integer", got)
		}
		if got := schemaTypeString(t, schema.Properties["sint32Value"].Type); got != "integer" {
			t.Fatalf("sint32Value.type = %q, want integer", got)
		}
		if got := schemaTypeString(t, schema.Properties["sfixed32Value"].Type); got != "integer" {
			t.Fatalf("sfixed32Value.type = %q, want integer", got)
		}
		if got := schemaTypeString(t, schema.Properties["uint32Value"].Type); got != "integer" {
			t.Fatalf("uint32Value.type = %q, want integer", got)
		}
		if got := schemaTypeString(t, schema.Properties["fixed32Value"].Type); got != "integer" {
			t.Fatalf("fixed32Value.type = %q, want integer", got)
		}
		if got := schemaTypeString(t, schema.Properties["int64Value"].Type); got != "string" {
			t.Fatalf("int64Value.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["sint64Value"].Type); got != "string" {
			t.Fatalf("sint64Value.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["sfixed64Value"].Type); got != "string" {
			t.Fatalf("sfixed64Value.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["uint64Value"].Type); got != "string" {
			t.Fatalf("uint64Value.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["fixed64Value"].Type); got != "string" {
			t.Fatalf("fixed64Value.type = %q, want string", got)
		}
		if got := schemaTypeString(t, schema.Properties["floatValue"].AnyOf[0].Type); got != "number" {
			t.Fatalf("floatValue.anyOf[0].type = %q, want number", got)
		}
		if got := schemaTypeString(t, schema.Properties["doubleValue"].AnyOf[0].Type); got != "number" {
			t.Fatalf("doubleValue.anyOf[0].type = %q, want number", got)
		}
		if !slices.Equal(schema.Properties["status"].Enum, []string{"REPORT_STATUS_OK", "REPORT_STATUS_FAILED"}) {
			t.Fatalf("status.enum = %v, want enum names without hidden NONE", schema.Properties["status"].Enum)
		}

		details := schema.Properties["details"]
		if got := schemaTypeString(t, details.Type); got != "object" {
			t.Fatalf("details.type = %q, want object", got)
		}
		samples := schema.Properties["samples"]
		if got := schemaTypeList(t, samples.Type); !slices.Equal(got, []string{"array", "null"}) {
			t.Fatalf("samples.types = %v, want [array null]", got)
		}
		sampleItems := decodeNestedSchema(t, samples.Items)
		if got := schemaTypeString(t, sampleItems.Type); got != "integer" {
			t.Fatalf("samples.items.type = %q, want integer", got)
		}
		if len(sampleItems.Examples) != 1 || exampleFloat64(t, sampleItems.Examples[0]) != -1 {
			t.Fatalf("samples.items.examples = %v, want [-1]", sampleItems.Examples)
		}
		if len(samples.Examples) != 1 || !slices.Equal(exampleFloat64Slice(t, samples.Examples[0]), []float64{-1}) {
			t.Fatalf("samples.examples = %v, want [[-1]]", samples.Examples)
		}

		if got := schemaTypeList(t, schema.Properties["optionalBoolFlag"].Type); !slices.Equal(got, []string{"boolean", "null"}) {
			t.Fatalf("optionalBoolFlag.types = %v, want [boolean null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalTextValue"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalTextValue.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalBytesValue"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalBytesValue.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalInt32Value"].Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("optionalInt32Value.types = %v, want [integer null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalSint32Value"].Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("optionalSint32Value.types = %v, want [integer null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalSfixed32Value"].Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("optionalSfixed32Value.types = %v, want [integer null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalUint32Value"].Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("optionalUint32Value.types = %v, want [integer null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalFixed32Value"].Type); !slices.Equal(got, []string{"integer", "null"}) {
			t.Fatalf("optionalFixed32Value.types = %v, want [integer null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalInt64Value"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalInt64Value.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalSint64Value"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalSint64Value.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalSfixed64Value"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalSfixed64Value.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalUint64Value"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalUint64Value.types = %v, want [string null]", got)
		}
		if got := schemaTypeList(t, schema.Properties["optionalFixed64Value"].Type); !slices.Equal(got, []string{"string", "null"}) {
			t.Fatalf("optionalFixed64Value.types = %v, want [string null]", got)
		}
		if len(schema.Properties["optionalFloatValue"].AnyOf) != 3 {
			t.Fatalf("optionalFloatValue.anyOf length = %d, want 3", len(schema.Properties["optionalFloatValue"].AnyOf))
		}
		if len(schema.Properties["optionalDoubleValue"].AnyOf) != 3 {
			t.Fatalf("optionalDoubleValue.anyOf length = %d, want 3", len(schema.Properties["optionalDoubleValue"].AnyOf))
		}
		optionalStatus := schema.Properties["optionalStatus"]
		if len(optionalStatus.AnyOf) != 2 {
			t.Fatalf("optionalStatus.anyOf length = %d, want 2", len(optionalStatus.AnyOf))
		}
		if !slices.Equal(optionalStatus.AnyOf[0].Enum, []string{"REPORT_STATUS_OK", "REPORT_STATUS_FAILED"}) {
			t.Fatalf("optionalStatus.anyOf[0].enum = %v, want enum names without hidden NONE", optionalStatus.AnyOf[0].Enum)
		}
		if got := schemaTypeString(t, optionalStatus.AnyOf[1].Type); got != "null" {
			t.Fatalf("optionalStatus.anyOf[1].type = %q, want null", got)
		}
	})
}

func decodeSchema(t *testing.T, raw string) schemaDocument {
	t.Helper()

	var schema schemaDocument
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	return schema
}

func decodeNestedSchema(t *testing.T, value any) schemaDocument {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal nested schema: %v", err)
	}

	var schema schemaDocument
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal nested schema: %v", err)
	}

	return schema
}

func schemaTypeString(t *testing.T, value any) string {
	t.Helper()

	stringValue, ok := value.(string)
	if !ok {
		t.Fatalf("schema type has type %T, want string", value)
	}

	return stringValue
}

func schemaTypeList(t *testing.T, value any) []string {
	t.Helper()

	switch typed := value.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, element := range typed {
			stringValue, ok := element.(string)
			if !ok {
				t.Fatalf("schema type list element has type %T, want string", element)
			}
			values = append(values, stringValue)
		}
		return values
	case nil:
		return nil
	default:
		t.Fatalf("schema type list has type %T, want []any", value)
		return nil
	}
}

func exampleString(t *testing.T, value any) string {
	t.Helper()

	stringValue, ok := value.(string)
	if !ok {
		t.Fatalf("example has type %T, want string", value)
	}

	return stringValue
}

func exampleObject(t *testing.T, value any) map[string]any {
	t.Helper()

	objectValue, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("example has type %T, want map[string]any", value)
	}

	return objectValue
}

func exampleArray(t *testing.T, value any) []any {
	t.Helper()

	arrayValue, ok := value.([]any)
	if !ok {
		t.Fatalf("example has type %T, want []any", value)
	}

	return arrayValue
}

func exampleStringSlice(t *testing.T, value any) []string {
	t.Helper()

	arrayValue := exampleArray(t, value)
	values := make([]string, 0, len(arrayValue))
	for _, element := range arrayValue {
		values = append(values, exampleString(t, element))
	}

	return values
}

func exampleFloat64(t *testing.T, value any) float64 {
	t.Helper()

	floatValue, ok := value.(float64)
	if !ok {
		t.Fatalf("example has type %T, want float64", value)
	}

	return floatValue
}

func exampleFloat64Slice(t *testing.T, value any) []float64 {
	t.Helper()

	arrayValue := exampleArray(t, value)
	values := make([]float64, 0, len(arrayValue))
	for _, element := range arrayValue {
		values = append(values, exampleFloat64(t, element))
	}

	return values
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}

	return true
}
