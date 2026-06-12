# Exploration: MCP Resources

## Намерение
Реализовать поддержку MCP Resources (третий серверный примитив MCP, spec 2025-11-25) в генераторе `protoc-gen-mcp`. Resources предоставляют данные, доступные агентам через URI-адресацию. Существуют два вида: **статические** (фиксированный URI, автоматически перечисляются в `resources/list`) и **шаблонные** (параметризованный URI по RFC 6570, перечисляются в `resources/templates/list`, клиент запрашивает конкретный экземпляр через resolved URI).

Это Phase 3 по ROADMAP — логическое продолжение Phase 2 (Prompts), следующий message-level примитив.

## Исследование

### Текущая кодовая база

Изучены ключевые файлы:

**1. Proto контракт** — [options.proto](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcp/options/v1/options.proto) (715+ строк):
- Extension numbers: 91001–91008 заняты (service, method, field, message, enum, enum_value, oneof, prompt).
- **91009 свободен** — зарезервирован для `resource` (ROADMAP § 3.1).
- `PromptOptions` (91008 на `MessageOptions`) — ближайший аналог: `name`, `title`, `description`, `icons`.
- `Icon { src, mime_type, sizes, theme }` — переиспользуется как есть.
- `ToolAnnotations` — для Tools (read_only_hint, destructive_hint, idempotent_hint, open_world_hint). **Не подходит для Resources** — у ресурсов `Annotations` содержит `audience` и `priority`, а не hints.
- **Нет существующих** `ResourceAnnotations` / `ResourceAudience` — требуется создать с нуля.

**2. Go SDK v1.6.0** — `github.com/modelcontextprotocol/go-sdk v1.6.0` ([go.mod](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/go.mod)):
- `server.AddResource(resource *mcp.Resource, handler ResourceHandler)` — статический ресурс
- `server.AddResourceTemplate(template *mcp.ResourceTemplate, handler ResourceHandler)` — шаблонный ресурс
- `ResourceHandler = func(ctx context.Context, req ReadResourceRequest) (*ReadResourceResult, error)`
- `ReadResourceRequest.Params.URI` — resolved URI (строка)
- `ReadResourceResult` содержит `[]Content` через `mcp.NewReadResourceResult()`
- `mcp.NewTextResourceContents(uri, text)` — создаёт текстовое содержимое
- `mcp.Resource { URI, Name, Description, MIMEType }` — структура для статического ресурса
- `mcp.ResourceTemplate { URITemplate, Name, Description, MIMEType }` — структура для шаблона

**3. IR модель** — [model.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/model.go) (69 строк):
- `FileModel` уже содержит `Services []ServiceModel` и `Prompts []PromptModel`.
- Необходимо добавить `Resources []ResourceModel`.
- `PromptModel` служит эталоном для `ResourceModel`.

**4. Collector** — [collect.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect.go) (238 строк):
- `CollectFileModel()` вызывает `collectPrompts(file)` и добавляет результат в `model.Prompts`.
- Аналогично нужно вызвать `collectResources(file)` → `model.Resources`.

**5. Collector промптов** — [collect_prompt.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_prompt.go) (119 строк):
- Сканирует messages на `(mcp.options.v1.prompt)` через `getPromptOptions()`.
- Валидирует поля (отклоняет oneof, map, repeated, message-typed).
- Извлекает аргументы с required/optional семантикой.
- **Для Resources:** аналогичный сканер, но без валидации полей (поля — output schema, допускаются все типы). Вместо этого валидация URI vs URI template.

**6. Metadata** — [metadata.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/metadata.go) (456 строк):
- `getPromptOptions()` извлекает extension через `getExtension(message.Desc.Options(), mcpoptionsv1.E_Prompt)`.
- По аналогии: `getResourceOptions()` через `mcpoptionsv1.E_Resource`.

**7. Generator dispatch** — [generator.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/generator.go) (295 строк):
- Проверяет `len(model.Services) == 0 && len(model.Prompts) == 0` для пропуска пустых файлов.
- Нужно добавить `len(model.Resources) == 0` в ту же проверку для всех языков (Go, Kotlin, Java, TypeScript, Python).

**8. Рендереры** (5 файлов):
- Go: `render_go.go` — промпты генерируются в строках 112-189, используют `server.AddPrompt()`.
- Python: `render_python.go` — промпты в строках 181-226, используют `@server.add_prompt()`.
- Kotlin: `render_kotlin.go` — промпты в строках 174-225, используют `server.addPrompt()`.
- Java: `render_java.go` — промпты в строках 195-306, используют `RegisteredPrompt`.
- TypeScript: `render_typescript.go` — промпты в строках 545-633, используют `ListPromptsRequestSchema`.

**9. Reference implementation** — [grpc-mcp-gateway/todo_service.pb.mcp.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/references/grpc-mcp-gateway/examples/proto/generated/go/todo/todopbv1/todo_service.pb.mcp.go):
- `s.AddResourceTemplate(&mcp.ResourceTemplate{...}, runtime.DefaultResourceHandler())` — шаблон
- `s.AddResource(&mcp.Resource{...}, runtime.DefaultAppResourceHandler(...))` — статический
- Handler принимает `ReadResourceRequest`, возвращает `*ReadResourceResult`.

**10. Runtime** — [mcpruntime/prompt.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcpruntime/prompt.go) (125 строк):
- `ParsePromptArguments(args, msg, requiredFields)` — парсинг строк в proto.
- Для Resources нужны: `ExtractURIParams(uri, template)` и `MarshalResourceContent(msg, mimeType, uri)`.

## Инструменты сборки
- **Оркестратор:** нет единого (стандартные Go-команды, `examples/Makefile`)
- **Test:** `go test ./... -count=1`
- **Build:** `go build ./cmd/protoc-gen-mcp`
- **Lint:** `easyp lint`
- **Generate:** `easyp generate` (для proto), `go generate ./...` (для тестовых данных)
- **Source:** `.github/workflows/tests.yml`, `go.mod`

## Рассмотренные варианты

### Вариант A: Типизированные хэндлеры с ProtoJSON-сериализацией (рекомендуемый)

- **Описание:** Хэндлер статического ресурса возвращает типизированное proto message (`*AppConfig`), а генерированный код сериализует его через ProtoJSON в `TextResourceContents`. Хэндлер шаблонного ресурса получает извлечённые из URI параметры как Go-аргументы и тоже возвращает proto message. Генератор создаёт `<File>ResourceHandler` интерфейс:
  ```go
  type ExampleResourceHandler interface {
      ReadAppConfig(ctx context.Context) (*AppConfig, error)
      ReadUserProfile(ctx context.Context, userID string) (*UserProfile, error)
  }
  ```
- **Плюсы:** Полная типобезопасность. Proto-first: message fields документируют структуру ответа. Автоматическая ProtoJSON-сериализация, пользователь не работает с сырыми `ResourceContents`. URI параметры типизированы.
- **Минусы:** Всегда JSON-контент (`text/plain` или `application/json`). Бинарный контент (blob) невозможен без дополнительного хэлпера. Для шаблонных ресурсов нужен `List*()` метод, который возвращает `[]mcp.Resource` — это SDK-тип, не proto.
- **Сложность:** Средняя.

### Вариант B: Сырые SDK-типы (pass-through хэндлеры)

- **Описание:** Хэндлер получает `*mcp.ReadResourceRequest` и возвращает `*mcp.ReadResourceResult` напрямую. Proto message носит чисто декоративный характер (документирует схему ответа, но не используется в сигнатуре).
  ```go
  type ExampleResourceHandler interface {
      ReadAppConfig(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
      ReadUserProfile(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
  }
  ```
- **Плюсы:** Максимальная гибкость — поддерживает text, blob, любой MIME-type. Пользователь полностью контролирует формат ответа. Простая реализация генератора (не нужен ProtoJSON-маршалинг).
- **Минусы:** Потеря типобезопасности — proto message не связано с возвращаемым значением. Пользователь вручную создаёт `ResourceContents`. Нарушает proto-first философию проекта (message поля не используются). URI параметры нужно извлекать вручную из `req.Params.URI`.
- **Сложность:** Низкая.

### Вариант C: Гибрид — типизированные хэндлеры + опциональный raw-режим

- **Описание:** По умолчанию как Вариант A (типизированные возвраты, ProtoJSON). Для бинарных ресурсов или нестандартных MIME-типов хэндлер может вернуть специальный `RawResourceResult` вместо proto message, минуя ProtoJSON-сериализацию.
- **Плюсы:** Типизированный путь для 95% случаев + escape hatch для blob.
- **Минусы:** Усложняет генератор и хэндлер-интерфейс. Дуальный return type в Go неудобен (нужен wrapper или `interface{}`).
- **Сложность:** Высокая.

## Рассмотренные варианты: роль полей message

### Вариант D: Поля message = output schema (рекомендуемый)

- **Описание:** Поля proto message описывают структуру ответа ресурса. Генерированный код сериализует заполненный message через ProtoJSON. Это *противоположно* Prompts (где поля = input args).
- **Плюсы:** Естественно для Resources — это *данные*, которые возвращаются. Позволяет документировать API. Обеспечивает типобезопасность.
- **Минусы:** URI-параметры шаблона (`{user_id}`) не проверяются на соответствие полям message на уровне генерации (они — входные параметры из URL, а не из message). Некоторые поля message могут совпадать с URI-параметрами, но это несовпадение по семантике.

### Вариант E: Пустой message + сырой handler

- **Описание:** Message помечается как `(mcp.options.v1.resource)`, но поля игнорируются. Handler полностью формирует ответ вручную.
- **Плюсы:** Простота. Нет путаницы в роли полей.
- **Минусы:** Теряется proto-first суть — message становится пустой заглушкой. Нет документации структуры ответа в proto-контракте.

## Рассмотренные варианты: List-хэндлер для шаблонных ресурсов

**Контекст:** Промпты уже используют SDK-типы в return value (`[]mcp.PromptMessage`). Прецедент установлен — аргумент "handler должен работать только с proto types" не валиден.

### Вариант F: Статический List при регистрации (рекомендуемый)

- **Описание:** Для `uri_template`-ресурсов генерируется `List<MessageName>s(ctx) ([]mcp.Resource, error)` — пользователь реализует перечисление доступных экземпляров. Вызывается **один раз** при `Register<File>Resources()`. Для каждого возвращённого экземпляра генерированный код вызывает `server.AddResource(instance, readHandler)`, что делает их видимыми в `resources/list`.
- **Плюсы:** Полноценная поддержка `resources/list` для шаблонных ресурсов. Стабильный handler interface — при появлении в SDK dynamic listing hook, перегенерация кода переключит на динамический вызов без изменений в пользовательском коде. Согласуется с паттерном Prompts (SDK-типы в return value).
- **Минусы:** Статический список — экземпляры, появившиеся после старта сервера, не попадут в `resources/list` до рестарта.
- **Сложность:** Средняя.

### Вариант G: Без List-хэндлера, только Read

- **Описание:** Для шаблонных ресурсов генерируется только `Read*()` хэндлер. Шаблоны перечисляются через `resources/templates/list` автоматически SDK. `resources/list` для шаблонов — ответственность пользователя (при необходимости).
- **Плюсы:** Проще. Меньше методов в интерфейсе.
- **Минусы:** Нет автоматического `resources/list` для конкретных экземпляров. Менее полная поддержка протокола.
- **Сложность:** Низкая.

## Ограничения и риски

- **Backward compatibility:** Добавление нового extension 91009 — аддитивное изменение, не ломает существующий код. Старый генератор игнорирует неизвестные extensions.
- **Annotations различаются:** `ToolAnnotations` (hints) ≠ `Annotations` (audience + priority). Нужны отдельные `ResourceAnnotations` и `ResourceAudience` enum. В MCP spec `Annotations` содержит `audience: Role[]` и `priority: number`.
- **Namespace и URI:** Namespace должен применяться только к `name` (display name для listing), но **не** к `uri` / `uri_template` — URI стабильный адрес, не зависящий от регистрации.
- **URI template parsing:** Нужен парсер `{param}` из RFC 6570 шаблона. Простой regex для MVP (`{[a-zA-Z_][a-zA-Z0-9_]*}`) покроет все практические случаи. Полный RFC 6570 (operators +, #, ., / и т.д.) — deferred.
- **Бинарный контент:** MCP протокол поддерживает два вида содержимого ресурсов: `TextResourceContents` (поле `text: string`) и `BlobResourceContents` (поле `blob: string`, base64). Go SDK предоставляет оба: `mcp.NewTextResourceContents(uri, text)` и `mcp.NewBlobResourceContents(uri, blob)`. При типизированном подходе (proto → ProtoJSON) мы всегда генерируем `TextResourceContents`. Blob-ресурсы (images, PDF и т.д.) пользователь может зарегистрировать через SDK напрямую — генерированные и ручные ресурсы сосуществуют на одном сервере. Для MVP это допустимое ограничение.
- **SDK consistency:** Go SDK `AddResource` / `AddResourceTemplate` подтверждены. Для Python, Kotlin, Java, TypeScript SDK — нужно проверить наличие эквивалентных API.

## Рекомендованное направление

**Архитектура хэндлеров:** Вариант A (типизированные хэндлеры с ProtoJSON) — согласуется с proto-first философией проекта, где proto message определяет контракт данных.

**Роль полей message:** Вариант D (поля = output schema) — естественная семантика для ресурсов.

**List-хэндлер:** Вариант F (статический List при регистрации) — включаем в v1. Интерфейс стабилен, при появлении dynamic listing hook в SDK — перегенерация кода адаптирует реализацию без изменений в пользовательском коде.

**Бинарный контент:** Deferred. Протокол поддерживает blob (base64), но для генерированного кода — всегда ProtoJSON → TextResourceContents. Пользователь может зарегистрировать blob-ресурсы через SDK напрямую.

Итого:
- В `options.proto` добавляются `ResourceOptions`, `ResourceAnnotations`, `ResourceAudience`, extension 91009.
- `ResourceModel` на уровне `FileModel` (аналогично `PromptModel`).
- `collect_resource.go` — сканер messages с fail-fast валидацией (uri XOR uri_template), парсинг `{param}` из шаблона.
- `mcpruntime/resource.go` — хэлперы: `ExtractURIParams()`, `MarshalResourceContent()`.
- 5 рендереров генерируют `<File>ResourceHandler` интерфейсы и `Register<File>Resources()`.
- Для шаблонных ресурсов: `List*()` метод (вызывается при регистрации) + `Read*()` метод.
- Для статических ресурсов: только `Read*()` метод.
- Golden тесты для каждого языка.

## Границы scope

- **Must-have (v1):**
  - Расширение `mcp.options.v1.resource` (91009) с `ResourceOptions`, `ResourceAnnotations`, `ResourceAudience`.
  - `ResourceModel` и `ResourceParamModel` в IR (`FileModel.Resources`).
  - `collectResources()` — сканирование messages, валидация uri XOR uri_template, парсинг `{param}` из шаблона.
  - Рендеринг `<File>ResourceHandler` интерфейсов и `Register<File>Resources()` для Go, Python, Kotlin, Java, TypeScript.
  - Типизированные `Read*()` хэндлеры (возвращают proto message → ProtoJSON-сериализация).
  - `List*()` хэндлеры для шаблонных ресурсов: вызываются при регистрации, возвращают `[]mcp.Resource` (SDK-тип, аналогично промптам, которые возвращают `[]mcp.PromptMessage`). Каждый экземпляр регистрируется через `server.AddResource()`.
  - Для шаблонных ресурсов: URI параметры извлекаются из `ReadResourceRequest.Params.URI` и передаются в хэндлер как строковые аргументы.
  - Runtime хэлперы: `ExtractURIParams()`, `MarshalResourceContent()`.
  - Golden тесты для 5 языков.
  - Namespace применяется к `name`, не к `uri` / `uri_template`.
  - Collector unit-тесты, runtime unit-тесты.

- **Deferred (v2):**
  - Dynamic listing: при появлении в SDK хука для динамического `resources/list` — перегенерация переключит `List*()` вызов со startup-only на per-request. **Добавить в ROADMAP** как отдельный пункт Phase 3.x.
  - Бинарный контент (blob) через escape hatch или отдельный handler-тип. Протокол поддерживает `BlobResourceContents` (base64), но генератор всегда использует `TextResourceContents`. Пользователь может зарегистрировать blob-ресурсы через SDK напрямую.
  - Полная поддержка RFC 6570 операторов (+, #, ., / и т.д.).
  - Stdio integration тесты для `resources/list`, `resources/templates/list`, `resources/read`.
  - Completions для URI-параметров (Phase 7).

- **Needs spike:**
  - Нет.

## Обновление ROADMAP

Требуется добавить в `ROADMAP.md` (Phase 3 или отдельный Phase 3.x):
- **Dynamic resource listing:** когда Go SDK добавит hook для динамического `resources/list`, обновить генератор — `List*()` handler вызывается при каждом запросе, а не только при startup. Handler interface не меняется.
- **Blob resource support:** escape hatch для бинарных ресурсов через отдельный handler-тип или конвенцию (single `bytes` field → `BlobResourceContents`).

## Предположения и открытые вопросы

- `[ASSUMPTION: SDK API]` Go SDK v1.6.0 `server.AddResource()` и `server.AddResourceTemplate()` стабильны и принимают `*mcp.Resource` / `*mcp.ResourceTemplate` + `ResourceHandler`.
- `[ASSUMPTION: URI template scope]` Для MVP достаточно простого парсинга `{param_name}` — без поддержки RFC 6570 operators, prefix, explode.
- `[ASSUMPTION: Output schema only]` Поля proto message описывают структуру ответа (output), а не входные параметры. URI template параметры — единственный input, они извлекаются из resolved URI, а не из proto полей.
- `[ASSUMPTION: Annotations structure]` MCP spec определяет `Annotations` для ресурсов как `{ audience: Role[], priority: number }`, что отличается от `ToolAnnotations`. Мы создаём отдельные типы `ResourceAnnotations` + `ResourceAudience` enum.
- `[ASSUMPTION: Namespace isolation]` Namespace применяется только к display `name` ресурса (для `resources/list`), но не к `uri` / `uri_template`, которые остаются стабильными адресами.
- `[ASSUMPTION: List at registration]` Статический List при `Register*()` вызове покрывает основной use case. При появлении dynamic listing в SDK — только перегенерация, без изменений в пользовательском коде.
- **Открытый вопрос:** Нужен ли `title` в `ResourceOptions`? В Go SDK `mcp.Resource` нет отдельного `Title` (только `Name` и `Description`). В `mcp.ResourceTemplate` тоже нет `Title`. Ответ: **не добавлять** `title` в MVP — у SDK нет соответствующего поля. Если MCP spec добавит `title`, расширим.
