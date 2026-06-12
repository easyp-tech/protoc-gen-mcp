# MCP Prompts — Requirements

**Status:** Draft
**Author:** AI Agent
**Date:** 2026-06-10

## Обзор

Генератор `protoc-gen-mcp` должен поддерживать второй серверный примитив MCP — **Prompts**. Prompt определяется через proto message с опцией `(mcp.options.v1.prompt)`. Поля сообщения становятся аргументами промпта. Генератор создаёт handler-интерфейс и функцию регистрации для каждого из 5 языков (Go, Python, Kotlin, Java, TypeScript). Аргументы промпта передаются клиентом как плоский `map[string]string` и парсятся рантаймом в типизированную proto-структуру.

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|------------|---------------|
| `PromptOptions` | Proto extension (91008) на `google.protobuf.MessageOptions`, маркирующая сообщение как MCP Prompt | `mcp/options/v1/options.proto` |
| `PromptModel` | IR-представление одного промпта в семантической модели генератора | `internal/codegen/model.go` |
| `PromptArgumentModel` | IR-представление одного аргумента промпта (поля сообщения) | `internal/codegen/model.go` |
| `PromptHandler` | Генерируемый интерфейс с методами-обработчиками промптов | Сгенерированные `*.mcp.go`, `*_mcp.py`, `*_mcp.kt`, `*.java`, `*_mcp.ts` |

## User Stories

- As a **proto-first MCP server developer**, I want to define MCP prompts as proto messages so that I get type-safe handler interfaces and registration code for my language.
- As a **multi-language team**, I want prompts to generate consistent handler APIs across Go, Python, Kotlin, Java, and TypeScript so that the developer experience is uniform.
- As a **proto author**, I want clear fail-fast errors when I use unsupported field types in a prompt message so that I fix issues at generation time, not at runtime.

---

## Требования

### Группа 1: Proto контракт

**REQ-1.1** WHEN proto файл содержит message с опцией `(mcp.options.v1.prompt)`, the system SHALL распознать это сообщение как MCP Prompt определение и включить его в генерируемый код.

**REQ-1.2** WHEN `PromptOptions.name` задан, the system SHALL использовать его как имя промпта. WHEN `PromptOptions.name` пуст, the system SHALL использовать `snake_case` от имени proto message.

**REQ-1.3** WHEN `PromptOptions.title` задан, the system SHALL передать его в сгенерированный код как человекочитаемый заголовок промпта.

**REQ-1.4** WHEN `PromptOptions.description` задан, the system SHALL передать его в сгенерированный код как описание промпта.

**REQ-1.5** WHEN `PromptOptions.icons` заданы, the system SHALL передать их в сгенерированный код как иконки промпта.

**REQ-1.6** WHEN расширение `prompt` (91008) применяется к `google.protobuf.MessageOptions`, the system SHALL корректно извлекать `PromptOptions` из дескриптора сообщения.

### Группа 2: Аргументы промпта

**REQ-2.1** WHEN поле proto message является singular (без `optional`), the system SHALL пометить соответствующий аргумент промпта как `required: true`.

**REQ-2.2** WHEN поле proto message помечено `optional`, the system SHALL пометить соответствующий аргумент промпта как `required: false`.

**REQ-2.3** WHEN поле proto message имеет скалярный тип (`string`, `int32`, `int64`, `uint32`, `uint64`, `sint32`, `sint64`, `fixed32`, `fixed64`, `sfixed32`, `sfixed64`, `float`, `double`, `bool`, `bytes`), the system SHALL принять его как допустимый аргумент промпта.

**REQ-2.4** WHEN поле proto message имеет тип `enum`, the system SHALL принять его как допустимый аргумент промпта.

**REQ-2.5** WHEN поле proto message имеет тип `message` (вложенное сообщение), the system SHALL отклонить его с fail-fast ошибкой генерации, указывающей имя поля и причину.

**REQ-2.6** WHEN поле proto message помечено `repeated`, the system SHALL отклонить его с fail-fast ошибкой генерации.

**REQ-2.7** WHEN поле proto message является `map`, the system SHALL отклонить его с fail-fast ошибкой генерации.

**REQ-2.8** WHEN поле proto message входит в `oneof`, the system SHALL отклонить его с fail-fast ошибкой генерации.

**REQ-2.9** WHEN поле имеет `(mcp.options.v1.field).description`, the system SHALL использовать его как описание аргумента промпта. WHEN описание не задано, the system SHALL использовать proto source comment на поле.

### Группа 3: Пространство имён и именование

**REQ-3.1** WHEN namespace передан при регистрации промптов, the system SHALL формировать финальное имя промпта как `{namespace}_{name}`, применяя ту же логику нормализации (dot-to-underscore), что и для tools.

**REQ-3.2** WHEN namespace пуст при регистрации, the system SHALL использовать `name` без prefix'а.

**REQ-3.3** WHEN финальное имя промпта содержит символы `.`, the system SHALL нормализовать их в `_` (аналогично tool names).

### Группа 4: Семантическая модель (IR)

**REQ-4.1** WHEN proto файл содержит одно или более prompt-сообщений, the system SHALL собрать `[]PromptModel` и добавить в `FileModel.Prompts`.

**REQ-4.2** WHEN proto файл не содержит ни одного prompt-сообщения, the system SHALL оставить `FileModel.Prompts` пустым и не генерировать prompt-специфичный код.

### Группа 5: Генерация кода (5 языков)

**REQ-5.1** WHEN `FileModel.Prompts` не пуст и `lang=go`, the system SHALL генерировать интерфейс `<File>PromptHandler` с методом на каждый промпт и функцию `Register<File>Prompts(server, impl, namespace...)`.

**REQ-5.2** WHEN `FileModel.Prompts` не пуст и `lang=python`, the system SHALL генерировать Protocol `<File>PromptHandler` и функцию `register_<file>_prompts(server, impl, *, namespace=None)`.

**REQ-5.3** WHEN `FileModel.Prompts` не пуст и `lang=kotlin`, the system SHALL генерировать interface `<File>PromptHandler` и функцию `register<File>Prompts(server, impl, namespace?)`.

**REQ-5.4** WHEN `FileModel.Prompts` не пуст и `lang=java`, the system SHALL генерировать nested interface `<File>PromptHandler` и метод `register<File>Prompts(transport, impl, namespace)`.

**REQ-5.5** WHEN `FileModel.Prompts` не пуст и `lang=typescript`, the system SHALL генерировать interface `<File>PromptHandler` и функцию `register<File>Prompts(server, impl, namespace?)`.

**REQ-5.6** WHEN обработчик промпта вызван, the system SHALL передать типизированное proto-сообщение (заполненное из `map[string]string` аргументов) в метод handler'а.

**REQ-5.7** WHEN обработчик промпта возвращает результат, the system SHALL обернуть его в нативный для SDK тип ответа промпта (Go: `*mcp.GetPromptResult`, и аналоги для других языков).

### Группа 6: Рантайм-парсинг аргументов (Go)

**REQ-6.1** WHEN аргумент промпта имеет тип `string`, the system SHALL присвоить строковое значение из `map[string]string` напрямую.

**REQ-6.2** WHEN аргумент промпта имеет числовой тип (`int32`, `int64`, `uint32`, `uint64`, `float`, `double` и их аналоги), the system SHALL распарсить строковое значение в соответствующий числовой тип.

**REQ-6.3** WHEN аргумент промпта имеет тип `bool`, the system SHALL распарсить строковое значение (`"true"`, `"false"`) в булево.

**REQ-6.4** WHEN аргумент промпта имеет тип `bytes`, the system SHALL декодировать строковое значение из base64.

**REQ-6.5** WHEN аргумент промпта имеет тип `enum`, the system SHALL найти значение перечисления по строковому имени (например, `"EXPERTISE_LEVEL_BEGINNER"`).

**REQ-6.6** WHEN строковое значение аргумента невалидно для целевого типа (например, `"abc"` для `int32`), the system SHALL вернуть ошибку `InvalidParams` клиенту.

**REQ-6.7** WHEN required-аргумент отсутствует в `map[string]string`, the system SHALL вернуть ошибку `InvalidParams` с указанием имени отсутствующего аргумента.

**REQ-6.8** WHEN optional-аргумент отсутствует в `map[string]string`, the system SHALL оставить соответствующее поле proto message в значении по умолчанию (zero-value).

### Группа 7: Тестирование и верификация

**REQ-7.1** WHEN генератор обрабатывает тестовый proto файл с prompt-сообщениями, the system SHALL генерировать код, совпадающий с golden файлами для каждого из 5 языков.

**REQ-7.2** WHEN запускается collector на prompt-сообщение с недопустимым типом поля, the system SHALL в unit-тестах подтвердить fail-fast ошибку.

**REQ-7.3** WHEN запускается рантайм-парсинг аргументов с валидными и невалидными строками, the system SHALL в unit-тестах подтвердить корректный парсинг и возврат ошибок соответственно.

---

## Топологический порядок

```
REQ-1.* → REQ-2.* → REQ-4.* → REQ-5.* → REQ-6.* → REQ-7.*
REQ-3.* (параллельно с REQ-4.*, используется в REQ-5.*)
```

**Причина:** Proto контракт (1.*) определяет структуру данных. Валидация аргументов (2.*) опирается на эту структуру. IR модель (4.*) собирает валидированные данные. Рендереры (5.*) используют IR. Рантайм (6.*) реализует парсинг для Go. Тесты (7.*) верифицируют всю цепочку.

---

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|----------|
| Test | `go test ./... -count=1` | go.mod |
| Build | `go build ./cmd/protoc-gen-mcp` | go.mod |
| Lint | `easyp lint` | easyp.yaml |
| Generate | `easyp generate` | easyp.yaml |
