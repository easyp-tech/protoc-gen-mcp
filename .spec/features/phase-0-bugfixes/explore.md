# Exploration: Phase 0 — Bugfixes

## Намерение

Закрыть два известных пробела в текущей кодовой базе `protoc-gen-mcp` перед добавлением новых MCP примитивов (Prompts, Resources):

1. **TaskSupport в Go renderer** — опция `ExecutionOptions.task_support` рендерится для Python/Kotlin/Java/TypeScript, но пропущена в Go.
2. **Tool name mangling** — длинные имена `namespace_MethodName` могут превышать 64 символа (лимит Claude Desktop / некоторых MCP клиентов), нет механизма усечения.

## Исследование

### Bug 1: TaskSupport в Go renderer

**Изучено:**
- [render_go.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_go.go) (236 строк) — строки 80-92 формируют `ToolSpec{}`. Поля: `Name`, `Title`, `Description`, `Namespace`, `InputSchemaJSON`, `OutputSchemaJSON`, `Annotations`, `Icons`, `NewRequest`, `NewResponse`, `Handler`. **Нет `TaskSupport`**.
- [register.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcpruntime/register.go) (211 строк) — `ToolSpec` struct (строки 26-38) также **не имеет поля TaskSupport**.
- [model.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/model.go) — `MethodModel.TaskSupport` (строка 40) есть в IR модели.
- Go SDK v1.6.0 `mcp.Tool` struct — **не имеет поля `Execution`/`TaskSupport`**. Поля: `Meta`, `Annotations`, `Description`, `InputSchema`, `Name`, `OutputSchema`, `Title`, `Icons`.
- Другие рендереры (Python/Kotlin/Java/TypeScript) — **все генерируют TaskSupport** через свои SDK, которые поддерживают это поле.

**Корневая причина:**
Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.6.0) ещё **не реализовал `TaskSupport` / `Execution`** в типе `mcp.Tool`. Поэтому Go renderer **корректно не генерирует** это поле — его просто некуда поставить.

Это **НЕ баг генератора** — это ограничение Go SDK. Когда Go SDK добавит поле `Execution` в `mcp.Tool`, нужно будет:
1. Добавить `TaskSupport` в `ToolSpec` struct в `mcpruntime/register.go`
2. Прокинуть его в `server.AddTool()` вызов
3. Добавить рендеринг в `render_go.go`

**Рекомендация:** отложить до обновления Go SDK. Создать tracking issue.

### Bug 2: Tool name mangling (64 символа)

**Изучено:**
- [options.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcpruntime/options.go) (62 строки) — `qualifyToolName()` (строка 34) конкатенирует `namespace + "_" + name`. `normalizeToolSegment()` (строка 47) заменяет `.` на `_`. **Нет проверки длины**.
- Все 5 рендереров — tool name формируется через collector (`collect.go`), name уже финальный к моменту рендеринга.
- Конкуренты: K1 использует SHA-1 хеш (6 chars), K2 использует SHA-256 base36 (10 chars) для усечения.

**Реальный сценарий:** `mycompany_platform_UserManagementService_GetUserAccountDetailsByExternalID` = 75 символов > 64.

**Текущее поведение:** имя передаётся как есть → Claude Desktop может отклонить инструмент.

## Инструменты сборки

- **Оркестратор:** нет (Makefile отсутствует в корне, есть `examples/Makefile`)
- **Test:** `go test ./... -count=1`
- **Build:** `go build ./cmd/protoc-gen-mcp`
- **Lint:** `easyp lint`
- **Generate:** `easyp generate` (для proto), `go generate ./...` (для testdata)
- **Source:** `go.mod`, `.github/workflows/tests.yml`, `examples/Makefile`

## Рассмотренные варианты

### Bug 1: TaskSupport в Go

#### Вариант A: Отложить (рекомендуемый)
- **Описание:** Не трогать Go renderer. Создать tracking issue. Вернуться когда Go SDK добавит `Execution` в `mcp.Tool`.
- **Плюсы:** нет бесполезного кода, нет workaround'ов
- **Минусы:** Go renderer временно отстаёт от Python/Kotlin/Java/TypeScript
- **Сложность:** нулевая

#### Вариант B: Workaround через `_meta`
- **Описание:** Вставить TaskSupport в `mcp.Tool.Meta` как custom поле.
- **Плюсы:** информация доступна клиентам
- **Минусы:** нестандартно, клиенты не поймут, ломает при обновлении SDK
- **Сложность:** низкая, но бессмысленная

### Bug 2: Tool name mangling

#### Вариант A: Truncate + SHA-256 hash suffix (рекомендуемый)
- **Описание:** Если `len(fullName) > 64`: усечь до `54` символов + `_` + SHA-256 base36 (9 chars) = 64 символа.
- **Плюсы:** детерминистично, уникально, совместимо с Claude Desktop, аналог K2
- **Минусы:** длинные имена теряют читаемость
- **Сложность:** низкая (1 функция, ~20 строк)

#### Вариант B: Truncate без хеша
- **Описание:** Просто обрезать до 64 символов.
- **Плюсы:** проще
- **Минусы:** **коллизии** — два разных метода могут усечься до одного имени
- **Сложность:** минимальная

#### Вариант C: Fail-fast на этапе генерации
- **Описание:** Генератор падает с ошибкой если имя > 64.
- **Плюсы:** пользователь узнаёт о проблеме рано
- **Минусы:** плохой UX — пользователь не может исправить (namespace задаётся в рантайме)
- **Сложность:** минимальная

## Ограничения и риски

- **Breaking change:** mangling меняет имена tool'ов. Если у пользователя уже есть клиенты, ссылающиеся на длинные имена — они сломаются. Но реально такие имена и так не работают в Claude Desktop.
- **Где применять mangling:** в `mcpruntime` (Go) и в каждом из 4 других рендереров (Python, Kotlin, Java, TypeScript), т.к. namespace задаётся в рантайме.
- **64 — это правильный лимит?** MCP spec не определяет лимит. 64 — это ограничение Claude Desktop. Стоит сделать лимит конфигурируемым или hardcoded?

## Рекомендованное направление

1. **TaskSupport Go:** Вариант A — **отложить**. Нет смысла делать workaround для поля, которого нет в SDK.
2. **Tool name mangling:** Вариант C — **fail-fast на этапе генерации**. Генератор проверяет длину `namespace + "_" + methodName` из proto options и падает с ошибкой если > 64 символов. Пользователь узнаёт о проблеме сразу и сокращает namespace/имя метода в proto.

## Границы scope

- **Must-have (v1):**
  - Валидация длины tool name на этапе генерации в `collect.go`
  - Проверка: `len(namespace + "_" + methodName) > 64` → ошибка генерации
  - Понятное сообщение об ошибке с указанием какое имя превышает лимит и на сколько
  - Unit-тесты: имена ≤ 64 → OK, имена > 64 → ошибка
  - Проверка работает для всех 5 языков (валидация на уровне collector'а, до рендеринга)

- **Deferred (v2):**
  - TaskSupport в Go (ждём Go SDK)
  - Runtime-fallback (truncate + hash) если пользователь переопределяет namespace в рантайме
  - Конфигурируемый maxLen через plugin option

## Предположения и открытые вопросы

- [ASSUMPTION: 64 символа — верный лимит] Claude Desktop использует 64, hardcoded в генераторе.
- [ASSUMPTION: fail-fast только при генерации] Валидация на этапе codegen из proto options. Если пользователь переопределяет namespace в рантайме и превышает лимит — это его ответственность (deferred: runtime fallback в v2).
- [ASSUMPTION: проверка учитывает default namespace из proto] Генератор знает namespace из `service` options и method name. Финальное имя = `namespace + "_" + methodName`.

**Открытый вопрос:** нет открытых вопросов. Scope ясен.
