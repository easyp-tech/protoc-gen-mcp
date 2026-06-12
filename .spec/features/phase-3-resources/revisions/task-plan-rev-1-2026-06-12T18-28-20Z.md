# MCP Resources — Task Plan

**Status:** Draft
**Author:** AI Agent
**Date:** 2026-06-12

---

**Test Style Source:** Tier 2
- Evidence: `internal/codegen/collect_prompt_test.go`, `mcpruntime/prompt_test.go`, `mcpruntime/register_test.go`
- Key patterns: `newTempProtogenPlugin(t, protoFiles, generateFile)` для collector tests; direct `t.Fatalf`/`t.Errorf` assertions; inline proto source в test string constants; table-driven subtests с `t.Run`

**Commands:**

| Action   | Command                           | Source      |
|----------|-----------------------------------|-------------|
| Test     | `go test ./... -count=1`          | design §2.8 |
| Build    | `go build ./cmd/protoc-gen-mcp`   | design §2.8 |
| Lint     | `easyp lint`                      | design §2.8 |
| Generate | `easyp generate`                  | design §2.8 |

**Work type:** Pure feature — новое поведение без предшествующей реализации.

---

## Coverage Matrix

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-2 | CP-6 |
| REQ-1.2 | T-2 | CP-12 |
| REQ-1.3 | T-2 | CP-12 |
| REQ-1.4 | T-2 | CP-6 |
| REQ-1.5 | T-1, T-3 | CP-1 |
| REQ-1.6 | T-1, T-3 | CP-2 |
| REQ-2.1 | T-1, T-3 | CP-6 |
| REQ-2.2 | T-1, T-3 | CP-6 |
| REQ-2.3 | T-1, T-3 | CP-3 |
| REQ-2.4 | T-1, T-5 | CP-10 |
| REQ-3.1 | T-1, T-3 | CP-7 |
| REQ-3.2 | T-1, T-3 | CP-7 |
| REQ-3.3 | T-1, T-3 | CP-8 |
| REQ-4.1 | T-1, T-5 | CP-13 |
| REQ-4.2 | T-1, T-5 | CP-13 |
| REQ-4.3 | T-1, T-5 | CP-13 |
| REQ-4.4 | T-1, T-5 | CP-11 |
| REQ-4.5 | T-1, T-5 | CP-13 |
| REQ-4.6 | T-1, T-5 | CP-5 |
| REQ-4.7 | T-1, T-5 | CP-11 |
| REQ-4.8 | T-1, T-5 | CP-9 |
| REQ-4.9 | T-1, T-5 | CP-12 |
| REQ-4.10 | T-1, T-4 | CP-5 |
| REQ-5.1 | T-1, T-5 | CP-13 |
| REQ-5.2 | T-1, T-5 | CP-11 |
| REQ-5.3 | T-1, T-5 | CP-13 |
| REQ-6.1 | T-1, T-5 | CP-13 |
| REQ-6.2 | T-1, T-5 | CP-11 |
| REQ-7.1 | T-1, T-5 | CP-13 |
| REQ-7.2 | T-1, T-5 | CP-11 |
| REQ-8.1 | T-1, T-5 | CP-13 |
| REQ-8.2 | T-1, T-5 | CP-11 |
| REQ-9.1 | T-1, T-4 | CP-3 |
| REQ-9.2 | T-1, T-4 | CP-4 |
| REQ-9.3 | T-1, T-4 | CP-5 |
| REQ-10.1 | T-7 | CP-13 |
| REQ-10.2 | T-7 | CP-13 |
| REQ-11.1 | T-1 | CP-6 |
| REQ-11.2 | T-1 | CP-1, CP-2, CP-7 |

---

## T-1: GREEN — Написать тесты для collector и runtime ресурсов

*_Requirements: REQ-1.5, REQ-1.6, REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-3.1, REQ-3.2, REQ-3.3, REQ-4.1–REQ-4.10, REQ-5.1–REQ-5.3, REQ-6.1–REQ-6.2, REQ-7.1–REQ-7.2, REQ-8.1–REQ-8.2, REQ-9.1, REQ-9.2, REQ-9.3, REQ-11.1, REQ-11.2_*
*_Test_Style: `internal/codegen/collect_prompt_test.go`, `mcpruntime/prompt_test.go`_*
*_Complexity: standard_*

GOAL: Создать полный набор тестов ДО реализации. Тесты не должны компилироваться (типы ещё не существуют).

### T-1.1: Создать `internal/codegen/collect_resource_test.go`

Файл: `internal/codegen/collect_resource_test.go`

Написать тесты по паттерну `collect_prompt_test.go`, используя `newTempProtogenPlugin(t, protoFiles, generateFile)`:

- `TestCollectResources_RecognizesStaticResource` — proto message с `option (mcp.options.v1.resource) = {uri: "config://app", name: "app_config"}` → `model.Resources` содержит 1 элемент с `IsTemplate=false`, `URI="config://app"`, `Name="app_config"`. CP-6.
- `TestCollectResources_RecognizesTemplateResource` — proto message с `uri_template: "users://{user_id}/profile"` → `IsTemplate=true`, `URITemplate` задан, `Params` содержит `{Name: "user_id"}`. CP-6.
- `TestCollectResources_SkipsNonResourceMessage` — message без option → `model.Resources` пуст. CP-6.
- `TestCollectResources_DefaultNameSnakeCase` — пустой `name` → `Name == toSnakeCase("AppConfig") == "app_config"`. CP-8.
- `TestCollectResources_DefaultMIMEType` — пустой `mime_type` → `MIMEType == "application/json"`. CP-6.
- `TestCollectResources_DescriptionFromComment` — пустой `description`, proto leading comment `// App configuration.` → `Description == "App configuration."`. CP-6.
- `TestCollectResources_RejectsBothURIAndTemplate` — оба `uri` и `uri_template` заданы → error содержит `"mutually exclusive"`. CP-1.
- `TestCollectResources_RejectsNeitherURINorTemplate` — оба пустые → error содержит `"either uri or uri_template"`. CP-2.
- `TestCollectResources_RejectsTemplateWithoutParams` — `uri_template: "static://no-params"` → error содержит `"no parameters"`. CP-7.
- `TestCollectResources_RejectsInvalidParamIdentifier` — `uri_template: "x://{123bad}"` → error содержит `"invalid parameter"`. CP-7.
- `TestCollectResources_MultipleParamsExtracted` — `uri_template: "org://{org_id}/users/{user_id}"` → `Params` содержит 2 элемента: `org_id`, `user_id`. CP-3.
- `TestCollectResources_AnnotationsPropagated` — `ResourceAnnotations` с `audience: [USER]`, `priority: 0.8` → соответствующие поля в `ResourceModel`. CP-12.

### T-1.2: Создать `mcpruntime/resource_test.go`

Файл: `mcpruntime/resource_test.go`

Написать тесты для runtime хэлперов:

- `TestExtractURIParams_SimpleTemplate` — `ExtractURIParams("users://alice", "users://{id}")` → `map[string]string{"id": "alice"}`. CP-3.
- `TestExtractURIParams_MultipleParams` — `ExtractURIParams("org://acme/users/bob", "org://{org}/users/{user}")` → `{"org": "acme", "user": "bob"}`. CP-3.
- `TestExtractURIParams_MismatchedURI` — URI не соответствует шаблону → error. CP-4.
- `TestExtractURIParams_EmptyParam` — URI с пустым значением параметра → error. CP-4.
- `TestMarshalResourceContent_ProtoJSON` — proto message → `[]mcp.ResourceContents` с `TextResourceContents`, содержимое — валидный JSON. CP-5.
- `TestMarshalResourceContent_CustomMIMEType` — MIME type пробрасывается в `ResourceContents`. CP-5.

---

## T-2: CODE — Добавить proto contract в `options.proto`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4_*
*_Preservation: CP-6, CP-12_*
*_Complexity: mechanical_*

GOAL: Добавить proto определения для MCP Resources.

### T-2.1: Добавить `ResourceAudience` enum, `ResourceAnnotations` message, `ResourceOptions` message в `options.proto`

Файл: `mcp/options/v1/options.proto`

Добавить после блока `PromptOptions` (перед блоком `extend`):

```protobuf
enum ResourceAudience {
  RESOURCE_AUDIENCE_UNSPECIFIED = 0;
  RESOURCE_AUDIENCE_USER = 1;
  RESOURCE_AUDIENCE_ASSISTANT = 2;
}

message ResourceAnnotations {
  repeated ResourceAudience audience = 1;
  optional double priority = 2;
}

message ResourceOptions {
  string uri = 1;
  string uri_template = 2;
  string name = 3;
  string description = 4;
  string mime_type = 5;
  ResourceAnnotations annotations = 6;
  repeated Icon icons = 7;
}
```

### T-2.2: Добавить extension `resource = 91009` в `options.proto`

Файл: `mcp/options/v1/options.proto`

Добавить в блок `extend google.protobuf.MessageOptions`:

```protobuf
ResourceOptions resource = 91009;
```

### T-2.3: Перегенерировать Go code из proto

Запустить: `easyp generate`

CRITICAL: Сгенерированный Go код (`mcp/options/v1/*.pb.go`) должен содержать `E_Resource`, `ResourceOptions`, `ResourceAnnotations`, `ResourceAudience`.

### T-2.4: Проверить lint и build

Запустить: `easyp lint` и `go build ./cmd/protoc-gen-mcp`

---

## T-3: CODE — Реализовать collector ресурсов и IR model

*_Requirements: REQ-1.5, REQ-1.6, REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-3.1, REQ-3.2, REQ-3.3_*
*_Preservation: CP-1, CP-2, CP-3, CP-6, CP-7, CP-8, CP-10_*
*_Complexity: standard_*

GOAL: Добавить `ResourceModel` в IR и реализовать `collectResources()`.

### T-3.1: Добавить `ResourceModel` и `ResourceParamModel` в `model.go`

Файл: `internal/codegen/model.go`

Добавить после `PromptArgumentModel`:

```go
type ResourceModel struct {
    ProtoFullName string
    ProtoName     string
    Name          string
    Description   string
    URI           string
    URITemplate   string
    MIMEType      string
    IsTemplate    bool
    Params        []ResourceParamModel
    Annotations   *mcpoptionsv1.ResourceAnnotations
    Icons         []*mcpoptionsv1.Icon
    Output        TypeRef
}

type ResourceParamModel struct {
    Name string
}
```

Добавить поле `Resources []ResourceModel` в `FileModel` после `Prompts`.

### T-3.2: Добавить `getResourceOptions()` в `metadata.go`

Файл: `internal/codegen/metadata.go`

Добавить функцию по паттерну `getPromptOptions()`:
- Использует `getExtension(message.Desc.Options(), mcpoptionsv1.E_Resource)`
- Type assert к `*mcpoptionsv1.ResourceOptions`
- Возвращает `(*mcpoptionsv1.ResourceOptions, error)`

### T-3.3: Создать `collect_resource.go`

Файл: `internal/codegen/collect_resource.go`

Реализовать `collectResources(file *protogen.File) ([]ResourceModel, error)`:
1. Итерация по `file.Messages`
2. `getResourceOptions(message)` — пропуск если nil
3. Валидация: uri XOR uri_template (fail-fast error при обоих или ни одного)
4. Если `uri_template`: парсинг `{param}` через regex `\{([a-zA-Z_][a-zA-Z0-9_]*)\}`, fail-fast если нет параметров или невалидный identifier
5. Default `name` → `toSnakeCase(message.Desc.Name())`
6. Default `mime_type` → `"application/json"`
7. `description` fallback → `parseCommentBlock(message.Comments.Leading).Description`
8. Сборка `ResourceModel` с `Output: newTypeRef(message)`

### T-3.4: Интегрировать `collectResources()` в `collect.go`

Файл: `internal/codegen/collect.go`

Добавить после вызова `collectPrompts(file)` (строка ~127):

```go
resources, err := collectResources(file)
if err != nil {
    return FileModel{}, err
}
model.Resources = resources
```

### T-3.5: Запустить тесты collector

Запустить: `go test ./internal/codegen/... -count=1 -run TestCollectResources`

CRITICAL: Все тесты из T-1.1 должны проходить.

---

## T-4: CODE — Реализовать runtime хэлперы

*_Requirements: REQ-9.1, REQ-9.2, REQ-9.3, REQ-4.10_*
*_Preservation: CP-3, CP-4, CP-5_*
*_Complexity: standard_*

GOAL: Реализовать `ExtractURIParams()` и `MarshalResourceContent()` в `mcpruntime`.

### T-4.1: Создать `mcpruntime/resource.go`

Файл: `mcpruntime/resource.go`

Реализовать:

**`ExtractURIParams(uri, uriTemplate string) (map[string]string, error)`:**
1. Извлечь параметры из шаблона: `{param}` → имена
2. Заменить каждый `{param}` на `([^/]+)` для построения regex
3. Escape остальных частей шаблона через `regexp.QuoteMeta`
4. Собрать regex, match против URI
5. Если нет match → error `"uri %q does not match template %q"`
6. Для каждого captured group: если пустой → error `"parameter %q has empty value"`
7. Вернуть `map[paramName]value`

**`MarshalResourceContent(uri, mimeType string, msg proto.Message) ([]mcp.ResourceContents, error)`:**
1. `protojson.MarshalOptions{EmitDefaultValues: true}.Marshal(msg)` → JSON bytes
2. Создать `mcp.TextResourceContents{URI: uri, MIMEType: mimeType, Text: string(jsonBytes)}`
3. Вернуть `[]mcp.ResourceContents{&textContents}, nil`

### T-4.2: Запустить тесты runtime

Запустить: `go test ./mcpruntime/... -count=1 -run TestExtractURIParams && go test ./mcpruntime/... -count=1 -run TestMarshalResourceContent`

CRITICAL: Все тесты из T-1.2 должны проходить.

---

## T-5: CODE — Реализовать рендереры для 5 языков

*_Requirements: REQ-4.1–REQ-4.9, REQ-5.1–REQ-5.3, REQ-6.1–REQ-6.2, REQ-7.1–REQ-7.2, REQ-8.1–REQ-8.2_*
*_Preservation: CP-9, CP-10, CP-11, CP-12, CP-13_*
*_Complexity: complex_*

GOAL: Генерировать `ResourceHandler` интерфейсы и `RegisterResources()` для каждого языка.

### T-5.1: Обновить skip logic в `generator.go`

Файл: `internal/codegen/generator.go`

Во всех проверках вида `if len(model.Services) == 0 && len(model.Prompts) == 0` добавить `&& len(model.Resources) == 0`. Затронутые места:
- Go branch skip check
- Kotlin branch skip check
- Java branch skip check
- TypeScript branch skip check
- Python `pythonModelRequiresOutputForHandler` function

### T-5.2: Реализовать Go renderer для ресурсов в `render_go.go`

Файл: `internal/codegen/render_go.go`

Добавить блок `if len(model.Resources) > 0 { ... }` после блока промптов (после строки ~190):

1. Import SDK типы: `mcp.Resource`, `mcp.ResourceTemplate`, `mcp.ReadResourceRequest`, `mcp.ReadResourceResult`, `mcp.TextResourceContents`
2. Import runtime: `mcpruntime.ExtractURIParams`, `mcpruntime.MarshalResourceContent`
3. Генерировать `<FileGoName>ResourceHandler` interface:
   - Для static: `Read<ProtoName>(ctx context.Context) (*<OutputType>, error)`
   - Для template: `List<ProtoName>s(ctx context.Context) ([]mcp.Resource, error)` + `Read<ProtoName>(ctx context.Context, <param1> string, ...) (*<OutputType>, error)`
4. Генерировать `Register<FileGoName>Resources(ctx context.Context, server *mcp.Server, impl <Interface>, opts ...RegisterOption) error`:
   - Nil check impl
   - Resolve namespace через `resolveOptions`
   - Для static: `server.AddResource(&mcp.Resource{URI, Name, Description, MIMEType, Annotations}, readHandler)` — handler вызывает `impl.Read*()`, сериализует через `MarshalResourceContent()`
   - Для template: вызов `impl.List*s(ctx)` → цикл `server.AddResource(instance, readHandler)` → `server.AddResourceTemplate(&mcp.ResourceTemplate{...}, readHandler)` — handler вызывает `ExtractURIParams()` → `impl.Read*(ctx, params...)` → `MarshalResourceContent()`
   - Namespace → prefix к `Name` через `qualifyToolName` (переиспользование), не к URI/URITemplate

### T-5.3: Реализовать Python renderer для ресурсов в `render_python.go`

Файл: `internal/codegen/render_python.go`

Добавить блок ресурсов по паттерну prompt rendering:
1. Protocol class `<File>ResourceHandler` с `read_<snake_name>()` и `list_<snake_name>s()` (для шаблонных)
2. `register_<file>_resources(server, impl, *, namespace=None)` функция
3. Static: `server.add_resource(...)` с handler
4. Template: `impl.list_*()` → register instances + `server.add_resource_template(...)`

### T-5.4: Реализовать Kotlin renderer для ресурсов в `render_kotlin.go`

Файл: `internal/codegen/render_kotlin.go`

Добавить блок ресурсов по паттерну prompt rendering:
1. Interface `<File>ResourceHandler` с `read<Name>()` и `list<Name>s()` (для шаблонных)
2. `register<File>Resources(ctx, server, impl, namespace?)` function

### T-5.5: Реализовать Java renderer для ресурсов в `render_java.go`

Файл: `internal/codegen/render_java.go`

Добавить блок ресурсов по паттерну prompt rendering:
1. Nested interface `<File>ResourceHandler` внутри sidecar class
2. `register<File>Resources(ctx, transportProvider, impl, namespace)` static method

### T-5.6: Реализовать TypeScript renderer для ресурсов в `render_typescript.go`

Файл: `internal/codegen/render_typescript.go`

Добавить блок ресурсов по паттерну prompt rendering:
1. Interface `<File>ResourceHandler` с `read<Name>()` и `list<Name>s()` (для шаблонных)
2. `register<File>Resources(server, impl, namespace?)` function

---

## T-6: CODE — Добавить test fixtures с ресурсами в тестовые proto

*_Requirements: REQ-11.1, REQ-11.2_*
*_Preservation: CP-6, CP-13_*
*_Complexity: mechanical_*

GOAL: Добавить тестовые proto fixtures с ресурсами для golden tests.

### T-6.1: Добавить resource messages в тестовые proto fixtures

Файл: `internal/testproto/example/v1/example.proto` (или соответствующий test fixture proto)

Добавить 2 resource messages:
1. Static resource:
```protobuf
message AppConfig {
  option (mcp.options.v1.resource) = {
    uri: "config://app"
    name: "app_config"
    description: "Application configuration"
    mime_type: "application/json"
  };
  string version = 1;
  string environment = 2;
}
```
2. Template resource:
```protobuf
message UserProfile {
  option (mcp.options.v1.resource) = {
    uri_template: "users://{user_id}/profile"
    name: "user_profile"
    description: "User profile data"
  };
  string user_id = 1;
  string display_name = 2;
  string email = 3;
}
```

### T-6.2: Перегенерировать тестовый код

Запустить: `easyp generate` (с `easyp.test.yaml` конфигурацией)

### T-6.3: Запустить build

Запустить: `go build ./cmd/protoc-gen-mcp`

---

## T-7: GREEN + GATE — Обновить golden files и финальная верификация

*_Requirements: REQ-10.1, REQ-10.2_*
*_Preservation: CP-1, CP-2, CP-3, CP-4, CP-5, CP-6, CP-7, CP-8, CP-9, CP-10, CP-11, CP-12, CP-13_*
*_Complexity: standard_*

GOAL: Обновить golden files, запустить полный тестовый набор, убедиться что всё проходит.

### T-7.1: Обновить golden файлы для 5 языков

Запустить генератор с `-update` flag (или вручную обновить golden files):
- `testdata/golden/example_mcp.go.golden`
- `testdata/golden/example_mcp.py.golden`
- `testdata/golden/example_mcp.kt.golden`
- `testdata/golden/example_mcp.java.golden`
- `testdata/golden/example_mcp.ts.golden`

CRITICAL: Golden files должны содержать сгенерированные `ResourceHandler` интерфейсы и `RegisterResources()` функции.

### T-7.2: Запустить полный тестовый набор

Запустить: `go test ./... -count=1`

CRITICAL: Все тесты должны проходить — и новые (resource collector, runtime), и существующие (tools, prompts).

### T-7.3: Запустить lint и build

Запустить: `easyp lint` и `go build ./cmd/protoc-gen-mcp`

### T-7.4: Проверить, что proto generate чистый

Запустить: `easyp generate`

CRITICAL: Никаких diff после повторной генерации — generated code должен быть идемпотентным.
