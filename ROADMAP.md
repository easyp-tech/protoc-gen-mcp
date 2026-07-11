# ROADMAP: Full MCP Protocol Implementation

> Цель: покрыть **весь** MCP протокол (spec 2025-11-25) и обогнать оба конкурента.

## Текущее состояние vs конкуренты

| Фича | easyp | K1 (Machani) | K2 (Redpanda) |
|---|:---:|:---:|:---:|
| **Tools** (list, call) | ✅ 5 языков | ✅ Go | ✅ Go |
| **inputSchema** (JSON Schema) | ✅ | ✅ | ✅ |
| **outputSchema** | ✅ | ❌ | ✅ |
| **Structured content** | ✅ | ❌ | ✅ |
| **ToolAnnotations** | ✅ | ✅ | ❌ |
| **Icons** | ✅ | ❌ | ❌ |
| **TaskSupport** | ⚠️ 4/5 языков (Go пропущен) | ❌ | ❌ |
| **Namespace** | ✅ | ✅ | ❌ |
| **Hidden methods** | ✅ | ❌ | ❌ |
| **Validation constraints** | ✅ | ❌ | ❌ |
| **WKT (15 типов)** | ✅ | ⚠️ Частично | ⚠️ Частично |
| **$ref/$defs рекурсия** | ✅ | ❌ | ✅ |
| **Oneof** | ✅ | ❌ | ✅ |
| **Maps** | ✅ | ❌ | ✅ |
| **Multi-language** | ✅ Go/Py/Kt/Java/TS | ❌ Go only | ❌ Go only |
| Proto options (7 extensions) | ✅ | ⚠️ 6 | ⚠️ 2 |
| **Strict Schema (OpenAI)** | ❌ | ❌ | ✅ |
| **Prompts** | ✅ 5 языков | ⚠️ Заглушка | ❌ |
| **Resources** | ✅ Go + stubs 4 | ⚠️ Заглушка | ❌ |
| **Elicitation** | ❌ | ✅ | ❌ |
| **Progress notifications** | ❌ | ✅ | ❌ |
| **gRPC Forwarding** | ❌ | ✅ | ✅ |
| **Tool name mangling (64)** | ❌ | ✅ SHA-1 | ✅ SHA-256 |
| **Offline schema validation** | ❌ | ❌ | ✅ |
| **Oneof discriminator (strict)** | ❌ | ❌ | ✅ `which` |

---

## Phase 0: Quick Wins (3 дня)

### 0.1 TaskSupport в Go рендере

**Проблема**: Опция `ExecutionOptions.task_support` рендерится для Python/Kotlin/Java/TS,
но пропущена в Go.

**Файлы**:
- `internal/codegen/render_go.go` — добавить `TaskSupport` в шаблон генерации

**Тесты**:
- `internal/codegen/generator_test.go` — golden test обновление
- `testdata/golden/example_mcp.go.golden` — обновить golden

---

### 0.2 Tool Name Mangling (64 символа)

**Проблема**: Claude Desktop требует ≤64 символов в имени tool. Длинные
`namespace_ServiceName_MethodName` обрезаются. K1 использует SHA-1 (6 chars),
K2 — SHA-256 (10 chars).

**Решение**: Если итоговое имя > 64 символов, усечь до 54 + `_` + SHA-256 base36 (10 chars).

**Файлы**:
- `internal/codegen/tool_name.go` — [NEW] функция `MangleToolName(name string, maxLen int) string`
- `internal/codegen/tool_name_test.go` — [NEW] тесты с edge-cases
- `internal/codegen/collect.go` — вызов mangling после формирования имени
- Все рендереры (Go/Python/Kotlin/Java/TS) — имена уже приходят из collector'а

**Тесты**:
- Unit-тесты на mangling: < 64 → без изменений, > 64 → усечение + hash
- Детерминизм: одинаковый вход → одинаковый hash

---

## Phase 1: Strict Schema Profile (4-5 недель) 🔴

> Разблокирует совместимость с OpenAI Structured Outputs, Anthropic, Gemini strict mode.

### 1.1 Generator Param

```
--mcp_out=. --mcp_opt=lang=go,schema=strict
```

Значения: `standard` (по умолчанию, текущее поведение), `strict` (OpenAI-compatible).

**Файлы**:
- `internal/codegen/options.go` — добавить `SchemaProfile` в `Options`
- `internal/codegen/options_test.go` — парсинг `schema=strict`

### 1.2 Strict Schema Generation

**Ограничения OpenAI strict mode**:
- Нет `oneOf`, `allOf`, `anyOf`, `not`
- Нет `$ref`/`$defs`
- Все properties в `required`
- `additionalProperties: false` обязателен
- Максимум 5 уровней вложенности

**Изменения в `internal/schema/schema.go` (1256 строк)**:

| Конструкция | standard (сейчас) | strict (новый) |
|---|---|---|
| `oneof` | `oneOf` + `allOf` + `not` | `which` discriminator enum + flat properties |
| Рекурсия (`$ref`/`$defs`) | `$ref` → `$defs` | Inline до depth=3, затем `{"type":"string","description":"JSON-encoded..."}` |
| WKT `Struct`/`Value`/`ListValue` | Полная JSON Schema | `{"type":"string","description":"JSON-encoded struct"}` |
| optional fields | Не в `required` | В `required`, но с `"type":["string","null"]` |
| `additionalProperties` | Не ставится | `false` на каждом object |

**Файлы**:
- `internal/schema/schema.go` — добавить `SchemaProfile` в `BuildOptions`, альтернативные ветки для strict
- `internal/schema/schema_strict.go` — [NEW] strict-specific helpers: `flattenOneof()`, `inlineRecursion()`, `strictifyWKT()`
- `internal/schema/schema_strict_test.go` — [NEW] unit-тесты strict-режима

### 1.3 Symmetric Runtime Transform

Strict schema меняет форму JSON. Runtime должен конвертировать:
- **Decode**: strict JSON → native proto (lift `which` → oneof, parse string placeholders)
- **Encode**: proto → strict JSON (wrap oneof, stringify deep subtrees)

**Файлы**:
- `mcpruntime/transform.go` — [NEW] `DecodeStrictArguments(schemaProfile, inputJSON, messageDescriptor)` + `EncodeStrictResult(schemaProfile, message)`
- `mcpruntime/transform_test.go` — [NEW] round-trip тесты: strict JSON ↔ proto message
- `mcpruntime/register.go` — вызов transform'а когда `SchemaProfile == strict`

**Для других языков** (Python, Kotlin, Java, TypeScript):
- Аналогичный transform в генерированном коде каждого языка
- Python: `_decode_strict()`/`_encode_strict()` в `*_mcp.py`
- Kotlin/Java: static methods в sidecar классе
- TypeScript: exported functions в `*_mcp.ts`

### 1.4 Offline Schema Validation Tests

Валидация generated schemas без API ключей.

**Файлы**:
- `internal/schema/schema_validation_test.go` — [NEW]
  - Проверка: strict-режим schemas не содержат `oneOf`/`allOf`/`not`/`$ref`
  - Проверка: все properties в `required`
  - Проверка: `additionalProperties: false` везде
  - Валидация через `github.com/google/jsonschema-go/jsonschema`

---

## Phase 2: Prompts (3-4 недели) ✅

> Второй серверный примитив MCP. Описание prompt templates в proto messages.

### 2.1 Proto Contract

Новые additions в `mcp/options/v1/options.proto`:

```protobuf
// PromptOptions marks a proto message as an MCP Prompt.
// Fields of the message become prompt arguments.
// Singular fields → required arguments.
// Optional/repeated fields → optional arguments.
message PromptOptions {
  // name overrides the prompt name. Default: snake_case of message name.
  string name = 1;

  // title sets the human-readable title for the prompt.
  string title = 2;

  // description of the prompt shown to clients.
  string description = 3;

  // icons for the prompt.
  repeated Icon icons = 4;
}

extend google.protobuf.MessageOptions {
  // prompt marks a message as an MCP Prompt definition.
  PromptOptions prompt = 91008;
}
```

**Extension numbers**: продолжение серии 91001-91007 → `91008`.

**Пример использования**:

```protobuf
syntax = "proto3";
package myapp.v1;
import "mcp/options/v1/options.proto";

message CodeReview {
  option (mcp.options.v1.prompt) = {
    name: "code_review"
    description: "Analyze code quality and suggest improvements"
  };

  string code = 1 [(mcp.options.v1.field) = {
    description: "Source code to review"
  }];
  string language = 2;
  optional string focus_area = 3;
}

message Summarize {
  option (mcp.options.v1.prompt) = {
    name: "summarize"
    description: "Summarize a document in given style"
  };

  string text = 1;
  optional int32 max_sentences = 2;
}
```

### 2.2 Semantic Model

**Файлы**:
- `internal/codegen/model.go` — добавить `PromptModel` и `PromptArgumentModel`:

```go
// PromptModel represents a single MCP prompt derived from a proto message.
type PromptModel struct {
    ProtoFullName string
    ProtoName     string
    Name          string   // MCP prompt name (snake_case)
    Title         string
    Description   string
    Icons         []*mcpoptionsv1.Icon
    Arguments     []PromptArgumentModel
    Input         TypeRef  // reference to the proto message
}

type PromptArgumentModel struct {
    ProtoName   string
    Name        string // JSON/MCP name
    Description string
    Required    bool
}
```

- `internal/codegen/model.go` — расширить `FileModel`:

```go
type FileModel struct {
    // ... existing fields ...
    Prompts []PromptModel   // NEW
}
```

### 2.3 Collector

**Файлы**:
- `internal/codegen/collect_prompt.go` — [NEW] scan all messages in file for `(mcp.options.v1.prompt)` option
  - Extract `PromptOptions` from message options
  - Walk message fields → `PromptArgumentModel` (name, description, required by proto3 rules)
  - Reuse existing `FieldOptions` for descriptions
  - Validate: no nested messages as arguments (MCP prompt args are flat key-value)

### 2.4 Renderers (5 языков)

Каждый renderer получает `FileModel.Prompts` и генерирует:

1. **Handler interface** с методом на каждый prompt
2. **Register function** вызывающую `server.AddPrompt()`
3. **Internal handler** конвертирующий proto args → typed message → user handler → `[]PromptMessage`

| Язык | Файл | Handler | Register |
|---|---|---|---|
| Go | `render_go.go` | `<File>PromptHandler` interface | `Register<File>Prompts(server, impl, namespace...)` |
| Python | `render_python.go` | `<File>PromptHandler` Protocol | `register_<file>_prompts(server, impl, *, namespace=None)` |
| Kotlin | `render_kotlin.go` | `<File>PromptHandler` interface | `register<File>Prompts(server, impl, namespace?)` |
| Java | `render_java.go` | `<File>PromptHandler` interface | `register<File>Prompts(transport, impl, namespace)` |
| TypeScript | `render_typescript.go` | `<File>PromptHandler` interface | `register<File>Prompts(server, impl, namespace?)` |

**Go генерация (пример)**:

```go
// prompts_mcp.go (generated)

// PromptsPromptHandler defines handlers for MCP prompts.
type PromptsPromptHandler interface {
    CodeReview(ctx context.Context, args *CodeReview) ([]mcp.PromptMessage, error)
    Summarize(ctx context.Context, args *Summarize) ([]mcp.PromptMessage, error)
}

func RegisterPromptsPrompts(server *mcp.Server, impl PromptsPromptHandler, namespace ...string) error {
    ns := ""
    if len(namespace) > 0 { ns = namespace[0] }

    server.AddPrompt(mcp.NewPrompt(
        joinName(ns, "code_review"),
        mcp.WithPromptDescription("Analyze code quality and suggest improvements"),
        mcp.WithPromptArgument("code", mcp.RequiredArgument("Source code to review")),
        mcp.WithPromptArgument("language", mcp.RequiredArgument("")),
        mcp.WithPromptArgument("focus_area", mcp.Argument("")),
    ), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
        msg := &CodeReview{}
        // parse req.Params.Arguments → msg fields via ProtoJSON
        result, err := impl.CodeReview(ctx, msg)
        if err != nil { return nil, err }
        return &mcp.GetPromptResult{Messages: result}, nil
    })
    // ... Summarize ...
    return nil
}
```

### 2.5 Runtime

**Файлы**:
- `mcpruntime/prompt.go` — [NEW] helper utilities:
  - `ParsePromptArguments(args map[string]string, msg proto.Message)` — заполнить proto message из prompt args
  - Prompt arguments в MCP — это `map[string]string`, конвертация в typed proto

### 2.6 Тесты

- `internal/codegen/collect_prompt_test.go` — [NEW] collector unit-тесты
- `internal/codegen/generator_test.go` — расширить golden тестами для prompts
- `testdata/golden/prompts_mcp.go.golden` — [NEW]
- `testdata/golden/prompts_mcp.py.golden` — [NEW]
- `testdata/golden/prompts_mcp.kt.golden` — [NEW]
- `testdata/golden/prompts_mcp.java.golden` — [NEW]
- `testdata/golden/prompts_mcp.ts.golden` — [NEW]
- `internal/testproto/` — добавить тестовый `.proto` файл с prompt messages
- `mcpruntime/prompt_test.go` — [NEW] ParsePromptArguments тесты

---

## Phase 3: Resources (4-5 недель) ✅

> Третий серверный примитив MCP. Данные, доступные агентам через URI.

### 3.1 Proto Contract

Новые additions в `mcp/options/v1/options.proto`:

```protobuf
// ResourceOptions marks a proto message as an MCP Resource.
// Fields of the message define the resource schema.
message ResourceOptions {
  // uri — static URI for fixed resources (e.g. "config://app").
  // Mutually exclusive with uri_template.
  // When set: generator uses server.AddResource() → auto-listed in resources/list.
  string uri = 1;

  // uri_template — parameterized URI (e.g. "user://{user_id}/profile").
  // Mutually exclusive with uri.
  // When set: generator uses server.AddResourceTemplate() → listed in
  // resources/templates/list. User implements List*() for resources/list.
  string uri_template = 2;

  // name — human-readable display name.
  string name = 3;

  // description of the resource.
  string description = 4;

  // mime_type of the resource content (e.g. "application/json", "text/plain").
  string mime_type = 5;

  // icons for the resource.
  repeated Icon icons = 6;

  // annotations — audience and priority hints.
  ResourceAnnotations annotations = 7;
}

// ResourceAnnotations provides audience and priority hints.
message ResourceAnnotations {
  // audience specifies who this resource is intended for.
  repeated ResourceAudience audience = 1;
  // priority — relative importance (0.0 = lowest, 1.0 = highest).
  optional double priority = 2;
}

// ResourceAudience indicates the intended consumer of a resource.
enum ResourceAudience {
  RESOURCE_AUDIENCE_USER = 0;
  RESOURCE_AUDIENCE_ASSISTANT = 1;
}

extend google.protobuf.MessageOptions {
  // resource marks a message as an MCP Resource definition.
  ResourceOptions resource = 91009;
}
```

**Extension number**: `91009`.

**Пример использования**:

```protobuf
syntax = "proto3";
package myapp.v1;
import "mcp/options/v1/options.proto";

// Статический ресурс — фиксированный URI, автоматически в resources/list
message AppConfig {
  option (mcp.options.v1.resource) = {
    uri: "config://app"
    name: "Application Config"
    description: "Current application configuration"
    mime_type: "application/json"
  };

  string version = 1;
  string environment = 2;
  bool debug_mode = 3;
}

// Динамический ресурс — URI template, нужен List handler
message UserProfile {
  option (mcp.options.v1.resource) = {
    uri_template: "user://{user_id}/profile"
    name: "User Profile"
    description: "Profile data for a specific user"
    mime_type: "application/json"
  };

  string user_id = 1;
  string display_name = 2;
  string email = 3;
  repeated string roles = 4;
}

// Динамический ресурс с несколькими параметрами
message ProjectFile {
  option (mcp.options.v1.resource) = {
    uri_template: "project://{project_id}/files/{path}"
    name: "Project File"
    description: "Source file from a project repository"
    mime_type: "text/plain"
  };

  string project_id = 1;
  string path = 2;
  string content = 3;
}
```

### 3.2 Semantic Model

**Файлы**:
- `internal/codegen/model.go` — добавить:

```go
type ResourceModel struct {
    ProtoFullName string
    ProtoName     string
    Name          string // MCP display name
    Description   string
    MimeType      string
    Icons         []*mcpoptionsv1.Icon
    Annotations   *mcpoptionsv1.ResourceAnnotations
    Output        TypeRef

    // Static vs Template
    IsStatic    bool   // true if uri is set
    URI         string // for static resources
    URITemplate string // for template resources
    Params      []ResourceParamModel // extracted from uri_template
}

type ResourceParamModel struct {
    TemplateName string // e.g. "user_id" (from URI template)
    GoName       string // e.g. "UserID" (PascalCase)
    SnakeName    string // e.g. "user_id" (for Python)
}
```

- `internal/codegen/model.go` — расширить `FileModel`:

```go
type FileModel struct {
    // ... existing fields ...
    Prompts   []PromptModel   // from Phase 2
    Resources []ResourceModel // NEW
}
```

### 3.3 Collector

**Файлы**:
- `internal/codegen/collect_resource.go` — [NEW]
  - Scan all messages for `(mcp.options.v1.resource)` option
  - Parse URI template → extract `{param}` names → `ResourceParamModel`
  - Validate: `uri` и `uri_template` mutually exclusive
  - Validate: template params match message field names

### 3.4 Renderers (5 языков)

Каждый renderer генерирует разный handler interface в зависимости от static/template:

**Статический** (`uri`): только `Read*()` — list автоматический
**Динамический** (`uri_template`): `List*()` + `Read*(params...)`

| Язык | Handler (static) | Handler (template) | Register |
|---|---|---|---|
| Go | `Read<Msg>(ctx) (*Msg, error)` | `List<Msg>s(ctx) ([]mcp.Resource, error)` + `Read<Msg>(ctx, params...) (*Msg, error)` | `Register<File>Resources(server, impl)` |
| Python | `read_<msg>(self)` | `list_<msg>s(self)` + `read_<msg>(self, params...)` | `register_<file>_resources(server, impl, *, namespace=None)` |
| Kotlin | `read<Msg>(): Msg` | `list<Msg>s(): List<Resource>` + `read<Msg>(params...): Msg` | `register<File>Resources(server, impl, namespace?)` |
| Java | `read<Msg>(): Msg` | `list<Msg>s(): List<Resource>` + `read<Msg>(params...): Msg` | `register<File>Resources(transport, impl, namespace)` |
| TypeScript | `read<Msg>(): Msg` | `list<Msg>s(): Resource[]` + `read<Msg>(params...): Msg` | `register<File>Resources(server, impl, namespace?)` |

**Go генерация (пример)**:

```go
// resources_mcp.go (generated)

type ResourcesResourceHandler interface {
    // Static: auto-listed, only Read
    ReadAppConfig(ctx context.Context) (*AppConfig, error)

    // Template: user provides List + Read
    ListUserProfiles(ctx context.Context) ([]mcp.Resource, error)
    ReadUserProfile(ctx context.Context, userID string) (*UserProfile, error)

    ListProjectFiles(ctx context.Context) ([]mcp.Resource, error)
    ReadProjectFile(ctx context.Context, projectID string, path string) (*ProjectFile, error)
}

func RegisterResourcesResources(server *mcp.Server, impl ResourcesResourceHandler, namespace ...string) error {
    // Static resource: AddResource
    server.AddResource(mcp.NewResource(
        "config://app",
        "Application Config",
        mcp.WithResourceDescription("Current application configuration"),
        mcp.WithMIMEType("application/json"),
    ), func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
        msg, err := impl.ReadAppConfig(ctx)
        if err != nil { return nil, err }
        data, _ := protojson.Marshal(msg)
        return []mcp.ResourceContents{
            mcp.TextResourceContents{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
        }, nil
    })

    // Template resource: AddResourceTemplate
    server.AddResourceTemplate(mcp.NewResourceTemplate(
        "user://{user_id}/profile",
        "User Profile",
        mcp.WithTemplateDescription("Profile data for a specific user"),
        mcp.WithTemplateMIMEType("application/json"),
    ), func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
        userID := extractURIParam(req.Params.URI, "user://{user_id}/profile", "user_id")
        msg, err := impl.ReadUserProfile(ctx, userID)
        if err != nil { return nil, err }
        data, _ := protojson.Marshal(msg)
        return []mcp.ResourceContents{
            mcp.TextResourceContents{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
        }, nil
    })

    return nil
}
```

### 3.5 Runtime

**Файлы**:
- `mcpruntime/resource.go` — [NEW]
  - `ExtractURIParams(uri string, template string) map[string]string` — URI template matching
  - `MarshalResourceContent(msg proto.Message, mimeType string) (mcp.ResourceContents, error)`

### 3.6 Тесты

- `internal/codegen/collect_resource_test.go` — [NEW] collector unit-тесты
- `internal/codegen/generator_test.go` — расширить golden тестами
- `testdata/golden/resources_mcp.*.golden` — [NEW] для каждого языка
- `internal/testproto/` — добавить тестовый `.proto` с resource messages
- `mcpruntime/resource_test.go` — [NEW] URI template matching тесты
- Stdio integration test с `resources/list`, `resources/templates/list`, `resources/read`

### 3.7 Cleanup Backlog (из Code Review F-1..F-5)

> Эти пункты не блокируют, но должны быть устранены перед v0.6.0 release.

| ID | Severity | Что подчистить |
|----|----------|----------------|
| F-1 | nit | Убрать `mcpResourceContentsIdent` в `render_go.go:197` — лишний import trigger для `mcp.ResourceContents`, тип уже transitively imported |
| F-2 | nit | Убрать `_ = annotations` dead assignment в generated Go code — вставить annotations inline в `AddResource`/`AddResourceTemplate` struct literal |
| F-3 | minor | Добавить negative collector tests: `TestCollectResources_RejectsBothURI`, `_RejectsNeitherURI`, `_RejectsInvalidParam`, `_RejectsTemplateWithoutParams` |
| F-4 | minor | Создать отдельный `internal/codegen/collect_resource_test.go` с dedicated negative fixture тестами (запланированный в design как `[NEW]`) |
| F-5 | nit | Дедуплицировать regex pattern `\{([a-zA-Z_][a-zA-Z0-9_]*)\}` между `collect_resource.go` и `mcpruntime/resource.go` |
| — | minor | Полная реализация Python/Kotlin/Java/TypeScript resource registration (заменить stubs на рабочий код) |
| — | minor | Обновить AGENTS.md: добавить Resources в "Implemented" секцию |
| — | minor | Добавить пример сервера с ресурсами в `cmd/example-mcp-server` |
| — | minor | Рассмотреть замену Go SDK `server.AddResource()`/`server.AddResourceTemplate()` на более чистое API когда SDK обновится |

---

## Phase 4: Elicitation (2-3 недели) 🟡

> Сервер запрашивает подтверждение от пользователя перед выполнением tool.

### 4.1 Proto Contract

```protobuf
// ElicitationOptions configures a confirmation dialog shown before tool execution.
message ElicitationOptions {
  // message displayed to the user.
  string message = 1;
  // Schema for the confirmation form — proto message full name.
  // Fields of the referenced message define the form fields.
  // If empty, a simple accept/decline dialog is shown.
  string schema = 2;
}
```

Добавляется в существующий `MethodOptions`:

```protobuf
message MethodOptions {
  // ... existing fields 1-4, 10-12 ...

  // elicitation configures a confirmation dialog before tool execution.
  ElicitationOptions elicitation = 13;
}
```

Никаких новых extensions — поле внутри существующего `MethodOptions`.

### 4.2 Пример

```protobuf
rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse) {
  option (mcp.options.v1.method) = {
    description: "Permanently delete a user account"
    annotations: { destructive_hint: true }
    elicitation: {
      message: "Are you sure you want to delete this user? This cannot be undone."
    }
  };
};
```

### 4.3 Codegen

Генерированный handler wrapper вызывает `server.CreateElicitation()` **перед**
вызовом user handler'а. Если пользователь ответил `"decline"` → ранний return
с текстовым сообщением.

**Файлы**:
- `internal/codegen/model.go` — добавить `Elicitation *ElicitationModel` в `MethodModel`
- `internal/codegen/collect.go` — извлечь `ElicitationOptions` из method options
- Все 5 рендереров — обернуть handler dispatch в elicitation check

### 4.4 Тесты

- Golden тесты для каждого языка
- Stdio integration: tool call → elicitation request → accept → result

---

## Phase 5: Progress & Server Streaming (3-4 недели) 🟡

> Progress notifications для долгих операций. Server streaming RPC разрешён.

### 5.1 MVP Rules обновление

Текущее правило: «non-unary RPC → fail fast».
Новое правило: «client streaming и bidi streaming → fail fast. Server streaming → допускается
с `(mcp.options.v1.method)` annotation, конвертируется в progress notifications».

### 5.2 Proto Contract

Нет новых proto options. Используется существующий `ExecutionOptions.task_support`
+ server streaming RPC:

```protobuf
// Server streaming — каждое промежуточное сообщение = progress notification
rpc GenerateReport(ReportRequest) returns (stream ReportProgress) {
  option (mcp.options.v1.method) = {
    description: "Generate analytics report"
    execution: { task_support: TASK_SUPPORT_OPTIONAL }
  };
};

message ReportProgress {
  oneof payload {
    ProgressUpdate progress = 1;
    ReportResponse result = 2;
  }
}

message ProgressUpdate {
  double fraction = 1;  // 0.0 - 1.0
  string message = 2;
}
```

### 5.3 Codegen

Для server streaming RPC:
1. Handler interface принимает `ProgressReporter` в контексте
2. Генерированный код конвертирует stream → `notifications/progress` + final `CallToolResult`

**Для unary RPC** (простой progress через context):

```go
type ProgressReporter interface {
    Report(progress float64, total float64, message string) error
}
```

Handler получает reporter через context:

```go
func (h *Handler) GenerateReport(ctx context.Context, req *pb.ReportRequest) (*pb.ReportResponse, error) {
    reporter := mcpruntime.ProgressFromContext(ctx)
    reporter.Report(0.25, 1.0, "Loading data...")
    // ... work ...
    reporter.Report(1.0, 1.0, "Done")
    return resp, nil
}
```

### 5.4 Файлы

- `mcpruntime/progress.go` — [NEW] `ProgressReporter` interface + context helpers
- `internal/codegen/collect.go` — detect server streaming RPC
- `internal/codegen/model.go` — `IsStreaming bool` в `MethodModel`
- Все 5 рендереров — streaming handler template

---

## Phase 6: gRPC Forwarding (3-4 недели) 🟡

> MCP сервер как прокси к существующему gRPC backend.

### Что это

AI Agent → MCP Client → **MCP Server (прокси)** → gRPC → Существующий API.

Пользователь уже имеет gRPC сервис. Генератор создаёт «мост» — клиент вызывает
MCP tool, а за кулисами идёт gRPC вызов к существующему серверу. Не нужно писать
handler — forwarding code делает всё автоматически.

### Generator Param

```
--mcp_out=. --mcp_opt=lang=go,mode=forward
```

### Generated Code

```go
// Forward<Service>Tools registers MCP tools that forward to the gRPC client.
func ForwardExampleAPITools(server *mcp.Server, client examplepb.ExampleAPIClient, opts ...mcpruntime.Option) error {
    // Each tool → unmarshal JSON → gRPC call → marshal response → MCP result
}
```

### Файлы

- `internal/codegen/options.go` — добавить `Mode` в `Options` (standard/forward)
- `internal/codegen/render_go_forward.go` — [NEW] Go forwarding renderer
- Тесты: e2e с in-process gRPC server + MCP forwarding

---

## Phase 7: Advanced Features (2-3 месяца) 🟢

### 7.1 Sampling (2 недели)

Server → Client: «вызови LLM для меня». Нишевая фича.

Proto: новый `SamplingOptions` на method → генерированный код вызывает `server.CreateSampling()`.

### 7.2 Tasks / Async Workflows (3 недели)

Experimental в MCP spec. Длительные операции с task handle.
Proto опция `TaskSupport` уже есть. Runtime: task lifecycle management.

### 7.3 Completions (1-2 недели)

Автодополнение аргументов prompt/resource.
Proto: новый `CompletionOptions` на field → генерированный handler для `completion/complete`.

### 7.4 Audio/Image Content (1 неделя)

Бинарный контент в tool/prompt responses. Расширить return types handlers.

### 7.5 OAuth 2.1 / Authorization (3-4 недели)

Middleware, не codegen. Runtime: token validation, CIMD support.

---

## Сводная таблица

| # | Phase | Статус | Усилие | Release |
|---|-------|--------|--------|---------|
| **2** | Prompts | ✅ Done | — | v0.5.0 |
| **3** | Resources (Go full, 4 lang stubs) | ✅ Done + cleanup backlog | — | v0.6.0 |
| **R1** | **Замена SDK на собственный MCP runtime** | 🔴 Планируется | 4-6 нед. | v0.7.0 |
| **T1** | **Улучшенный тест-пайплайн** | 🔴 Планируется | 2-3 нед. | v0.7.0 |
| **0** | Quick Wins (TaskSupport Go, Tool Name Mangling) | 🔴 | 3 дня | v0.7.1 |
| **1** | Strict Schema Profile (OpenAI) | 🔴 | 4-5 нед. | v0.8.0 |
| **4** | Elicitation | 🟡 explore | 2-3 нед. | v0.9.0 |
| **5** | Progress & Streaming | 🟡 explore | 3-4 нед. | v0.10.0 |
| **6** | gRPC Forwarding | 🟡 explore | 3-4 нед. | v0.11.0 |
| **7** | Advanced (Sampling, Tasks, Completions, Audio, OAuth) | 🟢 | 2-3 мес. | v0.12.0 |

---

## Phase R1: Замена SDK на собственный MCP runtime 🔴

> Убрать зависимость от `github.com/modelcontextprotocol/go-sdk/mcp` в generated code и runtime.
> Генерированный код использует собственный `mcpruntime` — полный контроль над API surface,
> совместимость и стабильность.

### Мотивация

- Go SDK (v0.8.0) — pre-stable, API ломается между версиями
- `server.AddResource()` / `server.AddResourceTemplate()` — ограничения для codegen
- Generated code зависит от SDK struct layout (Resources, Annotations, Role) — хрупко
- Собственный runtime = полный контроль над wire format, JSON-RPC, transport

### Scope

1. **MCP JSON-RPC layer** — stdio transport, request/response routing
2. **Tool/Prompt/Resource registration** — typed, codegen-friendly API
3. **Server capabilities** — `initialize`, `tools/list`, `tools/call`, `prompts/list`, `prompts/get`, `resources/list`, `resources/read`, `resources/templates/list`
4. **Schema validation** — reuse existing `internal/schema` + `mcpruntime`
5. **Annotations, Icons, Namespace** — first-class support

### Не входит (v1+)

- ~~SSE/Streamable HTTP transport (stdio first)~~ → Streamable HTTP done in Go
  `mcpruntime` (POST/GET/DELETE, sessions, SSE, Last-Event-ID). Legacy HTTP+SSE
  still out of scope.
- Client SDK (only server)
- OAuth/auth middleware

### Файлы

- `mcpruntime/server.go` — [NEW] MCP server core
- `mcpruntime/transport.go` — [NEW] stdio JSON-RPC transport
- `mcpruntime/types.go` — [NEW] MCP protocol types (Request, Response, Resource, Tool, etc.)
- `mcpruntime/register.go` — [MODIFIED] migrate from SDK types to own types
- `internal/codegen/render_*.go` — [MODIFIED] emit imports from `mcpruntime` not SDK
- `go.mod` — remove `github.com/modelcontextprotocol/go-sdk` dependency

---

## Phase T1: Улучшенный тест-пайплайн 🔴

> Единый, надёжный, быстрый CI-пайплайн для всех языков и примеров.

### Scope

1. **Hermetic test environment** — все зависимости (node_modules, Python venv, JVM) bootstrapped в CI
2. **TypeScript compile gate** — `npm ci` + `tsc` в CI, не skip из-за отсутствия `node_modules`
3. **Python integration tests** — hermetic virtualenv, не зависит от global packages
4. **JVM compile gate** — Gradle build в CI для Java/Kotlin примеров
5. **Stdio end-to-end** — единый test harness для всех языков (Go, Python, TS, Java, Kotlin)
6. **Golden regeneration** — `make golden` target для всех golden файлов
7. **Coverage reporting** — code coverage в CI badge
8. **Pre-commit hooks** — lint + build + golden check локально

### Файлы

- `.github/workflows/tests.yml` — [MODIFIED] расширить CI matrix
- `Makefile` — [NEW или MODIFIED] `make test`, `make golden`, `make lint`
- `internal/codegen/*_test.go` — убрать skip'ы для TS тестов, добавить CI setup
- `scripts/ci-setup.sh` — [NEW] bootstrap all language dependencies

---

## Верификация каждого Phase

| Phase | Unit тесты | Golden тесты | Integration | Compile gate |
|---|---|---|---|---|
| 2 ✅ | `collect_prompt_test.go`, `prompt_test.go` | 5 goldens | stdio: `prompts/list`, `prompts/get` | JVM + Node |
| 3 ✅ | `collect_resource_test.go`, `resource_test.go` | 5 goldens | stdio: `resources/list`, `templates/list`, `read` | JVM + Node |
| R1 | Server core tests, transport tests | Updated goldens (no SDK imports) | stdio e2e all primitives | — |
| T1 | — | Golden regen CI job | Full stdio matrix (5 languages) | All gates green |
| 0 | `tool_name_test.go` | Updated goldens | — | — |
| 1 | `schema_strict_test.go`, `transform_test.go` | Updated goldens | `schema_validation_test.go` | — |
| 4 | Elicitation collect test | Updated goldens | stdio: elicitation flow | — |
| 5 | Progress context test | Updated goldens | stdio: progress notifications | — |
| 6 | Forward render test | New goldens | e2e: gRPC → MCP forwarding | — |
| 7 | Per-feature tests | Per-feature goldens | Per-feature integration | — |

---

## Post-v0.12.0: Направления развития

> v0.12.0 не финал. Дальнейшие направления:

- **v1.0.0** — стабилизация API, semver guarantees, migration guide
- **Multi-transport** — ~~Streamable HTTP (Go)~~; WebSocket; other languages
- **Client SDK generation** — генерация типизированных MCP клиентов из proto
- **Plugin ecosystem** — custom code generators через proto options
- **IDE integration** — Language Server Protocol bridge, MCP inspector
- **Performance** — zero-alloc JSON-RPC, connection pooling
- **Observability** — OpenTelemetry tracing, metrics
- **Federation** — multi-server routing, service mesh integration
