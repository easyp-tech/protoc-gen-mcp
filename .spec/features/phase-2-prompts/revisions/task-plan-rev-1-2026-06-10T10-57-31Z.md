# MCP Prompts — Task Plan

**Work Type:** Pure feature — новый примитив без предшествующей реализации.

---

**Test Style Source:** Tier 2
- Evidence: [collect_test.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_test.go) — table-driven subtests, `newTempProtogenPlugin()` helper для inline proto, `newExampleProtogenPlugin()` для файловых fixtures, `compiledTestFiles()`, assertions через `t.Fatalf`
- Evidence: [generator_test.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/generator_test.go) — golden test pattern с `testdata/golden/*.golden`
- Key patterns: `TestCollectFileModel_*`, `TestGenerate_*_Golden`, `newTempProtogenPlugin(t, map[string]string{...})`

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Test | `go test ./... -count=1` | go.mod / design §2.8 |
| Build | `go build ./cmd/protoc-gen-mcp` | go.mod / design §2.8 |
| Lint | `easyp lint` | easyp.yaml / design §2.8 |
| Generate | `easyp generate` | easyp.yaml / design §2.8 |

---

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-2, T-3 | CP-1 |
| REQ-1.2 | T-2, T-3 | CP-1 |
| REQ-1.3 | T-2, T-3 | CP-1 |
| REQ-1.4 | T-2, T-3 | CP-1 |
| REQ-1.5 | T-2, T-3 | CP-1 |
| REQ-1.6 | T-1, T-2 | CP-1 |
| REQ-2.1 | T-2, T-3, T-5 | CP-2 |
| REQ-2.2 | T-2, T-3, T-5 | CP-2 |
| REQ-2.3 | T-2, T-3 | CP-3 |
| REQ-2.4 | T-2, T-3 | CP-3 |
| REQ-2.5 | T-3, T-5 | CP-4 |
| REQ-2.6 | T-3, T-5 | CP-4 |
| REQ-2.7 | T-3, T-5 | CP-4 |
| REQ-2.8 | T-3, T-5 | CP-4 |
| REQ-2.9 | T-2, T-3 | CP-5 |
| REQ-3.1 | T-4 | CP-6 |
| REQ-3.2 | T-4 | CP-6 |
| REQ-3.3 | T-4 | CP-6 |
| REQ-4.1 | T-2, T-3 | CP-7 |
| REQ-4.2 | T-2, T-3 | CP-7 |
| REQ-5.1 | T-4, T-5 | CP-8 |
| REQ-5.2 | T-4, T-5 | CP-8 |
| REQ-5.3 | T-4, T-5 | CP-8 |
| REQ-5.4 | T-4, T-5 | CP-8 |
| REQ-5.5 | T-4, T-5 | CP-8 |
| REQ-5.6 | T-4 | CP-9 |
| REQ-5.7 | T-4 | CP-9 |
| REQ-6.1 | T-3, T-5 | CP-10 |
| REQ-6.2 | T-3, T-5 | CP-10 |
| REQ-6.3 | T-3, T-5 | CP-10 |
| REQ-6.4 | T-3, T-5 | CP-10 |
| REQ-6.5 | T-3, T-5 | CP-10 |
| REQ-6.6 | T-3, T-5 | CP-11 |
| REQ-6.7 | T-3, T-5 | CP-12 |
| REQ-6.8 | T-3, T-5 | CP-13 |
| REQ-7.1 | T-5 | CP-14 |
| REQ-7.2 | T-5 | CP-4 |
| REQ-7.3 | T-5 | CP-10, CP-11, CP-12, CP-13 |

---

## T-1: Добавить proto контракт `PromptOptions`

*_Requirements: REQ-1.1, REQ-1.6_*
*_Complexity: mechanical_*

GOAL: Определить `PromptOptions` message и extension 91008 в `options.proto`, перегенерировать Go код.

### Подзадачи

1. **`mcp/options/v1/options.proto`** — добавить message `PromptOptions` с полями `string name = 1`, `string title = 2`, `string description = 3`, `repeated Icon icons = 4`. Добавить `extend google.protobuf.MessageOptions { PromptOptions prompt = 91008; }`. Разместить после блока Tools, перед Resources (если ещё нет — после последнего extend). IMPORTANT: Номер extension строго 91008.

2. **Запустить `easyp generate`** — перегенерировать `mcp/options/v1/options.pb.go` и `mcp/options/v1/options_mcp.py` (и другие генерируемые файлы опций).

3. **Запустить `easyp lint`** — убедиться, что proto проходит линтер без ошибок.

4. **Запустить `go build ./cmd/protoc-gen-mcp`** — убедиться, что бинарник компилируется с новыми proto-типами.

---

## T-2: Реализовать IR модель и collector промптов

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.5, REQ-1.6, REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-2.9, REQ-4.1, REQ-4.2_*
*_Complexity: standard_*

GOAL: Добавить `PromptModel`/`PromptArgumentModel` в IR, реализовать collector для извлечения промптов из proto messages.

### Подзадачи

1. **`internal/codegen/model.go`** — добавить структуры:
   ```go
   type PromptModel struct {
       ProtoFullName string
       ProtoName     string
       Name          string
       Title         string
       Description   string
       Icons         []*mcpoptionsv1.Icon
       Arguments     []PromptArgumentModel
       Input         TypeRef
   }
   type PromptArgumentModel struct {
       ProtoName   string
       Name        string
       Description string
       Required    bool
   }
   ```
   Добавить поле `Prompts []PromptModel` в `FileModel`.

2. **`internal/codegen/metadata.go`** — добавить функцию `getPromptOptions(message *protogen.Message) (*mcpoptionsv1.PromptOptions, error)` по аналогии с существующими `getServiceOptions`/`getMethodOptions`. Извлечение через `proto.GetExtension(message.Desc.Options(), mcpoptionsv1.E_Prompt)`.

3. **`internal/codegen/collect_prompt.go`** — `[NEW]` создать файл с функцией `collectPrompts(file *protogen.File) ([]PromptModel, error)`:
   - Итерировать `file.Messages`
   - Вызвать `getPromptOptions(message)` — если nil, пропустить
   - Для каждого поля message: проверить тип (скаляр или enum → OK; message/repeated/map/oneof → `return error` с именем поля и причиной)
   - Заполнить `Name` из `PromptOptions.Name` или `snake_case(message.GoName)` если пусто
   - Заполнить `Title`, `Description`, `Icons` из `PromptOptions`
   - Для каждого поля: `Required = !field.Desc.HasOptionalKeyword()`, `Description` из `(mcp.options.v1.field).description` или leading comment
   - Вернуть `[]PromptModel`

4. **`internal/codegen/collect.go`** — в `CollectFileModel()` после сбора `Services`, вызвать `collectPrompts(file)` и присвоить `model.Prompts`. CRITICAL: вызов должен идти до return, чтобы промпты без сервисов тоже собирались.

5. **Запустить `go build ./cmd/protoc-gen-mcp`** — убедиться в компиляции.

---

## T-3: Реализовать runtime парсинг аргументов промпта

*_Requirements: REQ-6.1, REQ-6.2, REQ-6.3, REQ-6.4, REQ-6.5, REQ-6.6, REQ-6.7, REQ-6.8, REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-2.5, REQ-2.6, REQ-2.7, REQ-2.8_*
*_Preservation: CP-1, CP-2, CP-3_*
*_Complexity: complex_*

GOAL: Создать `mcpruntime/prompt.go` с `ParsePromptArguments` — рефлективный парсер `map[string]string` → proto message.

### Подзадачи

1. **`mcpruntime/prompt.go`** — `[NEW]` создать файл с функцией:
   ```go
   func ParsePromptArguments(args map[string]string, msg proto.Message, requiredFields []string) error
   ```
   Реализация через `protoreflect`:
   - Получить `msg.ProtoReflect().Descriptor().Fields()`
   - Для каждого required-поля проверить наличие в `args` → если нет, вернуть ошибку `fmt.Errorf("missing required argument %q", name)`
   - Для каждого ключа в `args`: найти field descriptor по JSON name, спарсить значение по `field.Kind()`:
     - `StringKind` → `protoreflect.ValueOfString(value)`
     - `Int32Kind`/`Sint32Kind`/`Sfixed32Kind` → `strconv.ParseInt(value, 10, 32)` → `ValueOfInt32`
     - `Int64Kind`/`Sint64Kind`/`Sfixed64Kind` → `strconv.ParseInt(value, 10, 64)` → `ValueOfInt64`
     - `Uint32Kind`/`Fixed32Kind` → `strconv.ParseUint(value, 10, 32)` → `ValueOfUint32`
     - `Uint64Kind`/`Fixed64Kind` → `strconv.ParseUint(value, 10, 64)` → `ValueOfUint64`
     - `FloatKind` → `strconv.ParseFloat(value, 32)` → `ValueOfFloat32`
     - `DoubleKind` → `strconv.ParseFloat(value, 64)` → `ValueOfFloat64`
     - `BoolKind` → `strconv.ParseBool(value)` → `ValueOfBool`
     - `BytesKind` → `base64.StdEncoding.DecodeString(value)` → `ValueOfBytes`
     - `EnumKind` → `field.Enum().Values().ByName(protoreflect.Name(value))` → `ValueOfEnum`; если nil → error
   - При ошибке парсинга → `return fmt.Errorf("invalid value %q for argument %q: %w", value, name, err)`

2. **`mcpruntime/prompt_test.go`** — `[NEW]` создать тесты:
   - `TestParsePromptArguments_StringField` — строковое значение напрямую
   - `TestParsePromptArguments_Int32Field` — `"42"` → int32(42)
   - `TestParsePromptArguments_BoolField` — `"true"` → true
   - `TestParsePromptArguments_EnumField` — `"EXPERTISE_LEVEL_BEGINNER"` → enum value
   - `TestParsePromptArguments_BytesField` — base64 → bytes
   - `TestParsePromptArguments_InvalidNumber` — `"abc"` для int32 → error
   - `TestParsePromptArguments_MissingRequired` — отсутствие required → error с именем поля
   - `TestParsePromptArguments_MissingOptional` — отсутствие optional → нет ошибки, zero-value
   - `TestParsePromptArguments_UnknownArg` — ключ без matching field → игнорируется (DiscardUnknown)
   NOTE: Тест использует proto из `internal/testproto` — нужен тестовый proto с промптами (см. T-5.1).

3. **Запустить `go test ./mcpruntime/... -count=1`** — убедиться, что все тесты проходят.

---

## T-4: Реализовать Go renderer для промптов

*_Requirements: REQ-5.1, REQ-5.6, REQ-5.7, REQ-3.1, REQ-3.2, REQ-3.3_*
*_Preservation: CP-1, CP-2, CP-3, CP-7_*
*_Complexity: standard_*

GOAL: Расширить `render_go.go` для генерации `PromptHandler` интерфейса и `RegisterPrompts` функции. Обновить `generator.go`.

### Подзадачи

1. **`internal/codegen/render_go.go`** — после цикла по `model.Services`, добавить цикл по `model.Prompts`:
   - Сгруппировать промпты по proto file → сгенерировать один интерфейс `<File>PromptHandler` с методами `<MessageGoName>(ctx context.Context, req *<MessageType>) ([]mcp.PromptMessage, error)` для каждого промпта
   - Сгенерировать функцию `Register<File>Prompts(server *mcp.Server, impl <File>PromptHandler, opts ...mcpruntime.RegisterOption)`:
     - Для каждого промпта: `server.AddPrompt(&mcp.Prompt{Name: ..., Description: ..., Title: ..., Icons: ..., Arguments: []*mcp.PromptArgument{...}}, handlerFunc)`
     - Handler: создать `req := &<MessageType>{}`, вызвать `mcpruntime.ParsePromptArguments(req.Params.Arguments, req, requiredFields)`, вызвать `impl.<Method>(ctx, req)`, обернуть результат в `&mcp.GetPromptResult{Messages: result}`
   - Использовать `qualifyTypeRef` для message types (по аналогии с tools)
   - Namespace: использовать `mcpruntime.QualifyToolName(namespace, promptName)` — та же логика нормализации

2. **`internal/codegen/generator.go`** — изменить условие пропуска файлов:
   - Go path (line ~109): заменить `len(model.Services) == 0` на `len(model.Services) == 0 && len(model.Prompts) == 0`
   - IMPORTANT: аналогичные проверки для Python (~line 261), Kotlin (~line 57), Java (~line 72), TypeScript (~line 87) тоже обновить.

3. **Запустить `go build ./cmd/protoc-gen-mcp`** — убедиться в компиляции.

---

## T-5: Добавить тестовый proto, тесты collector'а, golden тесты и рендереры для Python/Kotlin/Java/TypeScript

*_Requirements: REQ-7.1, REQ-7.2, REQ-7.3, REQ-5.2, REQ-5.3, REQ-5.4, REQ-5.5, REQ-2.5, REQ-2.6, REQ-2.7, REQ-2.8_*
*_Preservation: CP-1, CP-2, CP-3, CP-4, CP-7, CP-8_*
*_Complexity: complex_*

GOAL: Создать тестовый proto, unit-тесты collector'а, golden-тесты для 5 языков, расширить Python/Kotlin/Java/TypeScript рендереры.

### Подзадачи

1. **`internal/testproto/prompts/v1/prompts.proto`** — `[NEW]` создать тестовый proto файл с:
   - `message CodeReview` с `(mcp.options.v1.prompt)` — 2 required string, 1 optional string
   - `message Summarize` с `(mcp.options.v1.prompt)` — 1 required string, 1 optional string, 1 optional int32
   - `message ExplainError` с `(mcp.options.v1.prompt)` — 2 required string, 1 required enum
   - `enum ExpertiseLevel` с hidden zero-value
   - `message PlainMessage` без опции `(mcp.options.v1.prompt)` — для проверки что не-промпты игнорируются

2. **`internal/codegen/collect_prompt_test.go`** — `[NEW]` создать тесты:
   - `TestCollectPrompts_RecognizesPromptMessage` — CodeReview/Summarize/ExplainError → 3 PromptModel, PlainMessage пропущен
   - `TestCollectPrompts_ArgumentRequiredSingular` — singular поле → `Required: true`
   - `TestCollectPrompts_ArgumentOptional` — optional поле → `Required: false`
   - `TestCollectPrompts_AcceptsScalarAndEnumTypes` — все типы в fixture → нет ошибки
   - `TestCollectPrompts_RejectsMessageField` — inline proto с nested message полем → fail-fast error
   - `TestCollectPrompts_RejectsRepeatedField` — inline proto с repeated полем → fail-fast error
   - `TestCollectPrompts_RejectsMapField` — inline proto с map полем → fail-fast error
   - `TestCollectPrompts_RejectsOneofField` — inline proto с oneof → fail-fast error
   - `TestCollectPrompts_DefaultNameSnakeCase` — пустой `PromptOptions.name` → snake_case от message name
   - `TestCollectPrompts_FieldDescriptionFromOptions` — `(mcp.options.v1.field).description` → используется
   - Использовать `newTempProtogenPlugin(t, map[string]string{...})` для inline proto тестов

3. **`internal/codegen/render_python.go`** — добавить генерацию промптов по аналогии с tools:
   - `<File>PromptHandler` Protocol с методами `async def <prompt_name>(self, args: <MessageDataclass>) -> GetPromptResult`
   - `register_<file>_prompts(server, impl, *, namespace=None)` вызывающий `server.add_prompt()` для каждого промпта
   - Per-field парсинг из `arguments: dict[str, str]` в dataclass

4. **`internal/codegen/render_kotlin.go`** — добавить генерацию промптов:
   - `interface <File>PromptHandler` с suspend-методами
   - `fun register<File>Prompts(server: Server, impl: <File>PromptHandler, namespace: String? = null)` вызывающий `server.addPrompt()`

5. **`internal/codegen/render_java.go`** — добавить генерацию промптов:
   - Nested `interface <File>PromptHandler` в sidecar классе
   - `static void register<File>Prompts(McpServerTransportProvider transport, <File>PromptHandler impl, String namespace)`

6. **`internal/codegen/render_typescript.go`** — добавить генерацию промптов:
   - `interface <File>PromptHandler` с методами
   - `function register<File>Prompts(server: Server, impl: <File>PromptHandler, namespace?: string)` вызывающий `server.addPrompt()`

7. **Golden files** — запустить генерацию для тестового proto и сохранить результаты как golden:
   - `testdata/golden/prompts_mcp.go.golden`
   - `testdata/golden/prompts_mcp.py.golden`
   - `testdata/golden/prompts_mcp.kt.golden`
   - `testdata/golden/prompts_mcp.java.golden`
   - `testdata/golden/prompts_mcp.ts.golden`

8. **Запустить `go test ./... -count=1`** — все тесты (collector, golden, runtime) должны пройти.

---

## T-6: Финальная верификация

*_Requirements: все REQ-*_*
*_Complexity: mechanical_*

**Тип: GATE**

### Подзадачи

1. **`go test ./... -count=1`** — все тесты проходят.
2. **`go build ./cmd/protoc-gen-mcp`** — бинарник компилируется.
3. **`easyp lint`** — proto файлы проходят линтер.
4. **`easyp generate`** — генерация проходит без ошибок (если изменились proto).
5. **Ручная проверка** — просмотреть сгенерированные golden файлы, убедиться что:
   - `PromptHandler` интерфейс содержит методы для каждого промпта
   - `RegisterPrompts` функция вызывает `server.AddPrompt` для каждого промпта
   - Аргументы промпта корректно размечены `Required: true/false`
   - Namespace корректно применяется к имени промпта
