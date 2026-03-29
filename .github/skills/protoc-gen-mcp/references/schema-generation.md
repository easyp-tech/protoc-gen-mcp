# Schema Generation Reference

## Field Type → JSON Schema Mapping

| Proto Type | JSON Schema Type | Notes |
|---|---|---|
| `int32`, `sint32`, `sfixed32` | `integer` | — |
| `uint32`, `fixed32` | `integer`, `minimum: 0` | — |
| `int64`, `sint64`, `sfixed64` | `string` | ProtoJSON encodes as string |
| `uint64`, `fixed64` | `string` | ProtoJSON encodes as string |
| `float`, `double` | `number` | Accepts `"NaN"`, `"Infinity"`, `"-Infinity"` strings |
| `bool` | `boolean` | — |
| `string` | `string` | — |
| `bytes` | `string` | base64 encoding |
| `enum` | `string` | ProtoJSON enum name strings; hidden zero-values excluded |
| `message` | `object` | Nested schema; recursive via `$defs`/`$ref` |
| `repeated T` | `array` of T | — |
| `map<K, V>` | `object` with `additionalProperties` | Key type determines `propertyNames.pattern` |
| `oneof` | `oneOf` array | Discriminated union |

## Well-Known Types

| Proto Type | JSON Schema | ProtoJSON Shape |
|---|---|---|
| `google.protobuf.Timestamp` | `string` (format: `date-time`) | `"2024-01-01T00:00:00Z"` |
| `google.protobuf.Duration` | `string` | `"3.5s"` |
| `google.protobuf.FieldMask` | `string` | `"field1,field2"` |
| `google.protobuf.Struct` | `object` (free-form) | `{ "key": value }` |
| `google.protobuf.Value` | any JSON value | `true`, `1.0`, `"str"`, `null`, `[]`, `{}` |
| `google.protobuf.ListValue` | `array` | `[value, ...]` |
| `google.protobuf.Any` | `object` with `@type` | `{"@type": "type.googleapis.com/...", ...}` |
| `google.protobuf.Empty` | `object` (empty) | `{}` |
| `google.protobuf.*Value` wrappers | unwrapped scalar type | `42`, `"str"`, `true` |

## Map Key Patterns

| Key Type | `propertyNames.pattern` |
|---|---|
| `string` | (no constraint) |
| `int32`, `sint32`, `sfixed32` | `^-?[0-9]+$` |
| `uint32`, `fixed32`, `uint64`, `fixed64` | `^[0-9]+$` |
| `int64`, `sint64`, `sfixed64` | `^-?[0-9]+$` |
| `bool` | `^(true\|false)$` |

## Requiredness Decision Tree

```
Is the field...
├── proto3 `optional`?           → NOT required, nullable
├── `repeated`?                  → NOT required, nullable
├── `map`?                       → NOT required, nullable
├── Inside a `oneof`?            → NOT required (unless oneof has required=true)
├── Has FieldOptions.optional?   → NOT required, nullable
└── Singular (none of above)?    → REQUIRED
```

## Nullability Rules

For any field NOT in the `required` array:
- Schema wraps type with null: `"type": ["string", "null"]`
- Or uses `"oneOf": [<schema>, {"type": "null"}]` for complex types
- Runtime accepts explicit JSON `null` → treated as unset in ProtoJSON

## Recursive Messages

- First occurrence generates full schema in `$defs`
- Subsequent references use `$ref: "#/$defs/MessageName"`
- Prevents infinite schema expansion
- Example: `RecursiveNode` with `optional RecursiveNode child`

## ProtoJSON Special Cases

- `int64`/`uint64` → JSON string (`"123456789"`)
- `float`/`double` → JSON number, but `NaN`/`Infinity`/`-Infinity` as strings
- `bytes` → base64 string
- `enum` → string name (`"REPORT_STATUS_OK"`)
- `Timestamp` → RFC 3339 string
- `Duration` → seconds string with `s` suffix
