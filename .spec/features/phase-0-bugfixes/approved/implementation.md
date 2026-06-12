# Phase 0: Tool Name Length Validation — Implementation

**Status:** Done
**Date:** 2026-06-09

## Выполненные изменения

### `internal/codegen/collect.go`

1. Добавлен `import "strings"` (строка 5)
2. Добавлена константа `maxToolNameLength = 64` (строка 204)
3. Добавлена функция `validateToolNameLength(namespace, methodName string) error` (строка 209):
   - Нормализует `.` → `_` в обоих аргументах
   - Формирует `fullName` = `namespace + "_" + methodName` (или только `methodName` при пустом namespace)
   - Возвращает ошибку если `len(fullName) > 64` с указанием полного имени, длины и лимита
4. Добавлена helper-функция `normalizeSegment(segment string) string` (строка 229)
5. Вызов `validateToolNameLength` интегрирован в `CollectFileModel` (строка 99) — между формированием `methodModel` и `append` в `serviceModel.Methods`

### `internal/codegen/collect_test.go`

1. `TestValidateToolNameLength` — 7 table-driven subtests:
   - empty namespace + short method → OK
   - namespace+method = 64 → OK
   - namespace+method = 65 → error containing "65" и "64"
   - dots normalized → OK
   - no namespace + method 65 → error
   - no namespace + method 64 → OK
   - error contains max limit

2. `TestCollectFileModel_RejectsToolNameExceeding64Chars` — integration test с proto fixture:
   - namespace 50 chars + method 20 chars = 71 > 64
   - CollectFileModel возвращает ошибку "exceeds maximum length"

## Верификация

- `go test ./internal/codegen/... -count=1` — **все тесты PASS** (кроме pre-existing `TestTypeScriptGeneratedNodeServer*` требующего `npm ci`)
- `go build ./cmd/protoc-gen-mcp` — **OK**
- Все golden тесты (Go/Python/Kotlin/Java/TypeScript) — **PASS**
