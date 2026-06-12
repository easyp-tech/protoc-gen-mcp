# Phase 0: Tool Name Length Validation — Task Plan

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-1, T-3 | CP-1 (absence) |
| REQ-1.2 | T-1, T-3 | CP-2 (propagation) |
| REQ-1.3 | T-1, T-3 | CP-1 (absence) |
| REQ-1.4 | T-1, T-3 | CP-3 (equivalence) |
| REQ-1.5 | T-3 | CP-4 (absence) |
| REQ-1.6 | T-1, T-3 | CP-2 (propagation) |

## Тип работы

**Pure feature** — валидация длины tool name ранее не существовала.

**Test Style Source:** Tier 2
- Evidence: [collect_test.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_test.go) — table-driven subtests, `compiledTestFiles()`, `t.Fatalf`
- Key patterns: `TestCollectFileModel_*`, in-process `protocompile`

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Test | `go test ./internal/codegen/... -count=1` | design §2.8 |
| Build | `go build ./cmd/protoc-gen-mcp` | design §2.8 |

---

### T-1: GREEN — Написать тесты для validateToolNameLength `[GREEN]`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.6_*
*_Test_Style:_* [collect_test.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_test.go)
*_Complexity: mechanical_*

GOAL: Написать unit-тесты для функции `validateToolNameLength` до её реализации.

1. В `internal/codegen/collect_test.go` добавить `TestValidateToolNameLength` с table-driven subtests:
   - Subtest `"empty namespace, short method"`: namespace=`""`, methodName=`"Health"` → expect `nil`
   - Subtest `"namespace+method exactly 64"`: namespace=`"ns"` (2 chars), methodName= строка из 61 символа → fullName = 2+1+61 = 64 → expect `nil`
   - Subtest `"namespace+method 65 chars"`: namespace=`"ns"`, methodName= строка из 62 символов → fullName = 65 → expect error containing `"65"` и `"64"`
   - Subtest `"dots normalized before length check"`: namespace=`"my.company"`, methodName=`"Do"` → normalized = `"my_company_Do"` (13 chars) → expect `nil`
   - Subtest `"no namespace, long method 65"`: namespace=`""`, methodName= строка из 65 символов → expect error
   - Subtest `"no namespace, method exactly 64"`: namespace=`""`, methodName= строка из 64 символов → expect `nil`

2. CRITICAL: Тесты сначала **не скомпилируются** (функция не существует). Это ожидаемо — перейти к T-2 для создания stub.

---

### T-2: CODE — Создать validateToolNameLength stub `[CODE]`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4_*
*_Preservation: CP-1, CP-2, CP-3_*
*_Complexity: mechanical_*

GOAL: Создать минимальный stub чтобы тесты из T-1 скомпилировались и упали (GREEN→RED→GREEN цикл).

1. В `internal/codegen/collect.go` добавить:
   - Константу `maxToolNameLength = 64`
   - Функцию `validateToolNameLength(namespace, methodName string) error` — реализация: нормализовать `namespace` и `methodName` (заменить `.` на `_`), сформировать `fullName` как `namespace + "_" + methodName` (или только `methodName` если namespace пуст), проверить `len(fullName) > maxToolNameLength` → `fmt.Errorf("tool name %q exceeds maximum length: %d > %d", fullName, len(fullName), maxToolNameLength)`

2. Запустить `go test ./internal/codegen/... -count=1 -run TestValidateToolNameLength` — CRITICAL: все subtests должны пройти.

---

### T-3: CODE — Интегрировать валидацию в CollectFileModel `[CODE]`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.5, REQ-1.6_*
*_Preservation: CP-1, CP-2, CP-3, CP-4_*
*_Complexity: standard_*

GOAL: Вызвать `validateToolNameLength` в `CollectFileModel` так чтобы превышение длины прерывало генерацию.

1. В `internal/codegen/collect.go`, после строки 95 (формирование `methodModel`) и перед строкой 100 (`serviceModel.Methods = append`), добавить:
   ```
   if err := validateToolNameLength(serviceModel.Namespace, methodModel.Name); err != nil {
       return FileModel{}, fmt.Errorf("method %s: %w", method.Desc.FullName(), err)
   }
   ```

2. В `internal/codegen/collect_test.go` добавить `TestCollectFileModel_RejectsToolNameExceeding64Chars`:
   - Использовать `compiledTestFiles()` helper для создания proto fixture с namespace длиной 50 символов и method name длиной 20 символов (total > 64)
   - CRITICAL: нужен proto fixture с `option (mcp.options.v1.service) = { namespace: "<50 chars>" }` и method name > 14 символов
   - NOTE: если создание fixture сложно (требует proto компиляции), использовать integration subtest в существующем `TestCollectFileModel_ExampleAPI` чтобы проверить что существующие tool names ≤ 64.

3. Запустить `go test ./internal/codegen/... -count=1` — CRITICAL: все существующие тесты + новые должны пройти.

---

### T-4: GATE — Полная верификация `[GATE]`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.5, REQ-1.6_*
*_Complexity: mechanical_*

GOAL: Убедиться что всё работает и ничего не сломано.

1. Запустить `go test ./internal/codegen/... -count=1` — все тесты должны пройти.
2. Запустить `go build ./cmd/protoc-gen-mcp` — binary должен собраться.
3. Запустить `go test ./... -count=1` — полный test suite проекта.
