# MCP Resources — Requirements

**Status:** Draft
**Author:** AI Agent
**Date:** 2026-06-12

## Обзор

Добавление поддержки MCP Resources — третьего серверного примитива протокола MCP (spec 2025-11-25) — в генератор `protoc-gen-mcp`. Resources предоставляют данные, доступные агентам через URI-адресацию. Определяются через message-level proto option `(mcp.options.v1.resource)` с extension number 91009. Генерируются типизированные хэндлеры для 5 языков (Go, Python, Kotlin, Java, TypeScript) с ProtoJSON-сериализацией ответов. Поддерживаются два вида: статические (фиксированный URI) и шаблонные (параметризованный URI template).

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|-------------|---------------|
| `ResourceOptions` | Proto message с метаданными MCP-ресурса: URI/URI template, name, description, MIME type, annotations, icons | `mcp/options/v1/options.proto` |
| `ResourceAnnotations` | Proto message с метаданными аннотаций ресурса: audience и priority | `mcp/options/v1/options.proto` |
| `ResourceAudience` | Proto enum, определяющий целевую аудиторию ресурса (user, assistant) | `mcp/options/v1/options.proto` |
| `ResourceModel` | IR-модель ресурса в генераторе, аналог `PromptModel` | `internal/codegen/model.go` |
| `ResourceParamModel` | IR-модель параметра URI template (`{param}`) | `internal/codegen/model.go` |
| Статический ресурс | Ресурс с фиксированным URI, зарегистрированный через `AddResource()` | `mcpruntime/resource.go` |
| Шаблонный ресурс | Ресурс с параметризованным URI template, зарегистрированный через `AddResourceTemplate()` | `mcpruntime/resource.go` |

## User Stories

- As a **proto API author**, I want to annotate proto messages with `(mcp.options.v1.resource)` so that the generator produces typed resource handlers and registration code.
- As a **MCP server developer**, I want to implement typed `Read*()` handlers that return proto messages so that resource content is automatically serialized to ProtoJSON.
- As a **MCP server developer** with template resources, I want to implement `List*()` handlers so that concrete resource instances are discoverable through `resources/list`.
- As a **MCP client/agent**, I want to discover available resources through `resources/list` and `resources/templates/list` so that I know what data is available.

## Требования

### 1. Proto Options

**REQ-1.1** WHEN proto file `mcp/options/v1/options.proto` загружен, the system SHALL содержать message `ResourceOptions` с полями: `uri` (string), `uri_template` (string), `name` (string), `description` (string), `mime_type` (string), `annotations` (`ResourceAnnotations`), `icons` (repeated `Icon`).

**REQ-1.2** WHEN proto file `mcp/options/v1/options.proto` загружен, the system SHALL содержать message `ResourceAnnotations` с полями: `audience` (repeated `ResourceAudience`), `priority` (double).

**REQ-1.3** WHEN proto file `mcp/options/v1/options.proto` загружен, the system SHALL содержать enum `ResourceAudience` со значениями `RESOURCE_AUDIENCE_UNSPECIFIED`, `RESOURCE_AUDIENCE_USER`, `RESOURCE_AUDIENCE_ASSISTANT`.

**REQ-1.4** WHEN proto file `mcp/options/v1/options.proto` загружен, the system SHALL содержать extension `resource` с номером 91009, расширяющий `google.protobuf.MessageOptions` типом `ResourceOptions`.

**REQ-1.5** WHEN поле `uri` и поле `uri_template` оба заданы в `ResourceOptions`, the system SHALL отклонить генерацию с ошибкой, указывающей на взаимоисключающий характер полей.

**REQ-1.6** WHEN ни `uri`, ни `uri_template` не заданы в `ResourceOptions`, the system SHALL отклонить генерацию с ошибкой, требующей указать одно из полей.

### 2. IR Model

**REQ-2.1** WHEN `CollectFileModel()` обрабатывает proto file, the system SHALL сканировать все messages на наличие option `(mcp.options.v1.resource)` и добавлять найденные ресурсы в `FileModel.Resources`.

**REQ-2.2** WHEN message помечен как ресурс, the system SHALL создать `ResourceModel` с полями: имя, описание, URI или URI template, MIME type, является ли шаблонным, список параметров URI template, annotations, icons, ссылка на proto message.

**REQ-2.3** WHEN `uri_template` содержит параметры в формате `{param_name}`, the system SHALL извлечь все параметры и создать для каждого `ResourceParamModel` с именем параметра.

**REQ-2.4** WHEN proto file содержит только messages с `ResourceOptions` (нет Services и Prompts), the system SHALL генерировать выходной файл (не пропускать файл как пустой).

### 3. Collector Validation

**REQ-3.1** WHEN `uri_template` содержит параметры, не соответствующие формату `{identifier}` (где identifier = `[a-zA-Z_][a-zA-Z0-9_]*`), the system SHALL отклонить генерацию с ошибкой.

**REQ-3.2** WHEN `uri_template` не содержит ни одного параметра `{...}`, the system SHALL отклонить генерацию с ошибкой, рекомендуя использовать `uri` вместо `uri_template`.

**REQ-3.3** WHEN `name` не задан в `ResourceOptions`, the system SHALL использовать имя proto message как display name ресурса.

### 4. Go Renderer

**REQ-4.1** WHEN proto file содержит ресурсы, the system SHALL генерировать интерфейс `<File>ResourceHandler` с методами для каждого ресурса.

**REQ-4.2** WHEN ресурс статический (с `uri`), the system SHALL генерировать метод `Read<MessageName>(ctx context.Context) (*<MessageName>, error)` в интерфейсе хэндлера.

**REQ-4.3** WHEN ресурс шаблонный (с `uri_template`), the system SHALL генерировать метод `Read<MessageName>(ctx context.Context, <param1> string, ...) (*<MessageName>, error)` с параметрами из URI template.

**REQ-4.4** WHEN ресурс шаблонный, the system SHALL генерировать метод `List<MessageName>s(ctx context.Context) ([]mcp.Resource, error)` для перечисления экземпляров.

**REQ-4.5** WHEN proto file содержит ресурсы, the system SHALL генерировать функцию `Register<File>Resources(server *mcp.Server, impl <File>ResourceHandler, opts ...mcpruntime.ResourceOption) error`.

**REQ-4.6** WHEN `Register<File>Resources()` вызван для статического ресурса, the system SHALL вызвать `server.AddResource()` с сериализацией возврата хэндлера через ProtoJSON в `TextResourceContents`.

**REQ-4.7** WHEN `Register<File>Resources()` вызван для шаблонного ресурса, the system SHALL вызвать `impl.List<MessageName>s()`, зарегистрировать каждый экземпляр через `server.AddResource()`, и зарегистрировать шаблон через `server.AddResourceTemplate()`.

**REQ-4.8** WHEN namespace задан, the system SHALL применить его как префикс к display `name` ресурса (через underscore), но НЕ к `uri` и `uri_template`.

**REQ-4.9** WHEN `ResourceAnnotations` задан в proto options, the system SHALL пробросить `audience` и `priority` в `mcp.Resource.Annotations` / `mcp.ResourceTemplate.Annotations`.

**REQ-4.10** WHEN хэндлер `Read*()` возвращает proto message, the system SHALL сериализовать его через `protojson.Marshal()` и обернуть в `mcp.NewTextResourceContents(uri, jsonString)`.

### 5. Python Renderer

**REQ-5.1** WHEN proto file содержит ресурсы для `lang=python`, the system SHALL генерировать Protocol class `<File>ResourceHandler` с методами `read_<snake_case_name>()` и `list_<snake_case_name>s()` (для шаблонных).

**REQ-5.2** WHEN `Register<File>Resources()` вызван для шаблонного ресурса в Python, the system SHALL вызвать `impl.list_<name>s()` при регистрации и зарегистрировать каждый экземпляр.

**REQ-5.3** WHEN генерируется Python-код для ресурса, the system SHALL следовать паттерну `python_handler=dataclass` по умолчанию, аналогично промптам.

### 6. Kotlin Renderer

**REQ-6.1** WHEN proto file содержит ресурсы для `lang=kotlin`, the system SHALL генерировать interface `<File>ResourceHandler` с методами `read<MessageName>()` и `list<MessageName>s()` (для шаблонных).

**REQ-6.2** WHEN `register<File>Resources()` вызван для шаблонного ресурса в Kotlin, the system SHALL вызвать `impl.list<Name>s()` при регистрации и зарегистрировать каждый экземпляр.

### 7. Java Renderer

**REQ-7.1** WHEN proto file содержит ресурсы для `lang=java`, the system SHALL генерировать nested interface `<File>ResourceHandler` внутри top-level sidecar class с методами `read<MessageName>()` и `list<MessageName>s()` (для шаблонных).

**REQ-7.2** WHEN `register<File>Resources()` вызван для шаблонного ресурса в Java, the system SHALL вызвать `impl.list<Name>s()` при регистрации и зарегистрировать каждый экземпляр.

### 8. TypeScript Renderer

**REQ-8.1** WHEN proto file содержит ресурсы для `lang=typescript`, the system SHALL генерировать interface `<File>ResourceHandler` с методами `read<MessageName>()` и `list<MessageName>s()` (для шаблонных).

**REQ-8.2** WHEN `register<File>Resources()` вызван для шаблонного ресурса в TypeScript, the system SHALL вызвать `impl.list<Name>s()` при регистрации и зарегистрировать каждый экземпляр.

### 9. Runtime

**REQ-9.1** WHEN runtime получает URI и URI template, the system SHALL извлечь значения параметров из URI, сопоставляя их с `{param}` плейсхолдерами шаблона.

**REQ-9.2** WHEN параметр из URI template отсутствует в resolved URI, the system SHALL вернуть ошибку с указанием недостающего параметра.

**REQ-9.3** WHEN хэндлер возвращает proto message и URI, the system SHALL сериализовать message через ProtoJSON и обернуть в `TextResourceContents` с указанным URI и MIME type.

### 10. Golden Tests

**REQ-10.1** WHEN запущены golden тесты, the system SHALL проверять сгенерированный код для ресурсов на соответствие эталонным файлам для каждого из 5 языков (Go, Python, Kotlin, Java, TypeScript).

**REQ-10.2** WHEN эталонные файлы не совпадают с генерированным выводом, the system SHALL показать diff и завершиться с ошибкой.

### 11. Test Fixtures

**REQ-11.1** WHEN тестовые proto-фикстуры содержат messages с `(mcp.options.v1.resource)`, the system SHALL использовать их для проверки collector, renderer и runtime.

**REQ-11.2** WHEN proto-фикстуры содержат невалидные ResourceOptions (оба uri и uri_template, ни одного, шаблон без параметров), the system SHALL проверять fail-fast отклонение генерации.

## Топологический порядок

```
REQ-1.* → REQ-2.* → REQ-3.* → REQ-4/5/6/7/8.* → REQ-9.* → REQ-10/11.*
```

Причина: Proto options (1.*) должны существовать до collector (2.*, 3.*). Collector создаёт IR, от которого зависят рендереры (4-8.*). Runtime хэлперы (9.*) используются генерированным кодом. Golden тесты и фикстуры (10-11.*) проверяют всё вместе.

Рендереры (4-8.*) могут реализовываться параллельно.

## Открытые вопросы для Design

| Вопрос | Почему важно | Затрагивает |
|--------|-------------|-------------|
| Как именовать `ResourceOption` для `Register*` — `ResourceOption` или переиспользовать существующий `Option`? | Влияет на public API runtime пакета | REQ-4.5, REQ-9.* |
| Нужен ли `context.Context` параметр в `Register*Resources()` для вызова `List*()`? | List вызывается при регистрации, может требовать контекст для БД/сети | REQ-4.5, REQ-4.7 |
| Как обрабатывать ошибку из `List*()` при регистрации? | `Register*` может быть void или возвращать error | REQ-4.5, REQ-4.7 |

## Verification Commands

| Action   | Command                                    | Source        |
|----------|--------------------------------------------|---------------|
| Test     | `go test ./... -count=1`                   | explore.md    |
| Build    | `go build ./cmd/protoc-gen-mcp`            | explore.md    |
| Lint     | `easyp lint`                               | explore.md    |
| Generate | `easyp generate`                           | explore.md    |
