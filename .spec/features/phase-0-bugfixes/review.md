# Phase 0: Tool Name Length Validation — Review

**Status:** Complete
**Date:** 2026-06-09

## Итог

Добавлена fail-fast валидация длины MCP tool name на этапе кодогенерации.

## Изменённые файлы

| Файл | Тип | Строки |
|------|-----|--------|
| [collect.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect.go) | MODIFIED | +38 строк: константа `maxToolNameLength`, функции `validateToolNameLength`, `normalizeSegment`, вызов в `CollectFileModel` |
| [collect_test.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_test.go) | MODIFIED | +110 строк: `TestValidateToolNameLength` (7 subtests), `TestCollectFileModel_RejectsToolNameExceeding64Chars` |

## Трассировка требований

| REQ | Покрытие |
|-----|---------|
| REQ-1.1 (≤ 64 → OK) | `TestValidateToolNameLength/namespace+method_exactly_64`, `/no_namespace,_method_exactly_64` |
| REQ-1.2 (> 64 → error с деталями) | `TestValidateToolNameLength/namespace+method_65_chars`, `/error_contains_max_limit` |
| REQ-1.3 (пустой namespace) | `TestValidateToolNameLength/empty_namespace,_short_method` |
| REQ-1.4 (нормализация `.`) | `TestValidateToolNameLength/dots_normalized_before_length_check` |
| REQ-1.5 (единообразно для всех языков) | Валидация в collector'е, до рендеринга. Все golden тесты (Go/Python/Kotlin/Java/TypeScript) — PASS |
| REQ-1.6 (точечная ошибка) | `TestCollectFileModel_RejectsToolNameExceeding64Chars` |

## Верификация

- `go test ./internal/codegen/... -count=1` — **PASS** (0 FAIL, кроме pre-existing TS Node test)
- `go build ./cmd/protoc-gen-mcp` — **OK**
- Все golden тесты — **не затронуты**

## Известные ограничения

- Если пользователь переопределяет namespace в рантайме через `WithNamespace()` / `register*Tools(server, impl, namespace)` — проверка длины **не выполняется** в рантайме. Это deferred на v2.
- Лимит 64 hardcoded. Конфигурация через plugin option — deferred.
