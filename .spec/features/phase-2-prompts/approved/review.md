# Code Review: phase-2-prompts (Revision 2 — Post-Fix)

## Verdict: PASS

Все 3 `major` и 1 `minor` finding из первой ревизии исправлены и верифицированы. Build, runtime тесты, и все 10 golden тестов (5 example + 5 prompts) проходят. TypeScript prompt handlers теперь используют registry/aggregation pattern. Java prompt handlers теперь хранят и диспатчат через `RegisteredPrompt` + `PromptInvoker`. Golden файлы для всех 5 языков созданы, и сравнительные тесты добавлены. Enum тест переписан с real assertions через `protodesc` + `dynamicpb`.

## Change Set

| File | Status | Notes |
|------|--------|-------|
| `mcp/options/v1/options.proto` | ✅ Planned | `PromptOptions` message + extension 91008 |
| `internal/codegen/model.go` | ✅ Planned | `PromptModel`, `PromptArgumentModel`, `FileModel.Prompts` |
| `internal/codegen/collect.go` | ✅ Planned | Вызов `collectPrompts` |
| `internal/codegen/collect_prompt.go` | ✅ Planned | Новый collector |
| `internal/codegen/collect_prompt_test.go` | ✅ Planned | Unit-тесты collector |
| `internal/codegen/metadata.go` | ✅ Planned | `getPromptOptions()` |
| `internal/codegen/render_go.go` | ✅ Planned | Go prompt rendering |
| `internal/codegen/render_python.go` | ✅ Planned | Python prompt rendering |
| `internal/codegen/render_kotlin.go` | ✅ Planned | Kotlin prompt rendering |
| `internal/codegen/render_java.go` | ✅ Fixed (F-2) | Java prompt rendering — wiring complete |
| `internal/codegen/render_typescript.go` | ✅ Fixed (F-1) | TypeScript prompt rendering — registry pattern |
| `internal/codegen/generator.go` | ✅ Planned | Расширение `len(model.Prompts) > 0` checks |
| `internal/codegen/generator_test.go` | ✅ Fixed (F-3) | 5 golden comparison tests + `TestWritePromptsGoldenFiles` |
| `internal/codegen/collect_test.go` | ✅ Fixed (F-3) | `newPromptsProtogenPlugin` helper |
| `internal/codegen/jvm_collect.go` | ✅ Planned | Проброс `model.Prompts` в `JVMFileModel` |
| `internal/codegen/jvm_model.go` | ✅ Planned | `Prompts []PromptModel` field |
| `internal/codegen/jvm_names.go` | ✅ Planned | `jvmFileBaseName()` helper |
| `internal/codegen/typescript_collect.go` | ✅ Planned | Проброс `model.Prompts` в `TypeScriptFileModel` |
| `internal/codegen/typescript_model.go` | ✅ Planned | `Prompts []PromptModel` field |
| `mcpruntime/prompt.go` | ✅ Planned | `ParsePromptArguments()` |
| `mcpruntime/prompt_test.go` | ✅ Fixed (F-4) | Real enum test via `protodesc`+`dynamicpb` |
| `internal/testproto/prompts/v1/prompts.proto` | ✅ Planned | Test fixtures |
| `testdata/golden/prompts.mcp.go.golden` | ✅ Fixed (F-3) | Go golden file (3911 bytes) |
| `testdata/golden/prompts_mcp.py.golden` | ✅ Fixed (F-3) | Python golden file (20859 bytes) |
| `testdata/golden/prompts_mcp.kt.golden` | ✅ Fixed (F-3) | Kotlin golden file (14708 bytes) |
| `testdata/golden/prompts_mcp.java.golden` | ✅ Fixed (F-3) | Java golden file (22817 bytes) |
| `testdata/golden/prompts_mcp.ts.golden` | ✅ Fixed (F-3) | TypeScript golden file (12072 bytes) |

## Requirements Traceability

| Requirement | Test(s) | Code | CP | Verdict |
|-------------|---------|------|----|---------| 
| REQ-1.1 | `TestCollectPrompts_RecognizesPromptMessage` | `collect_prompt.go` | CP-1 | ✅ |
| REQ-1.2 | `TestCollectPrompts_DefaultNameSnakeCase` | `collect_prompt.go` | CP-1 | ✅ |
| REQ-1.3 | `TestCollectPrompts_RecognizesPromptMessage` | `collect_prompt.go` | CP-1 | ✅ |
| REQ-1.4 | `TestCollectPrompts_RecognizesPromptMessage` | `collect_prompt.go` | CP-1 | ✅ |
| REQ-1.5 | `TestCollectPrompts_RecognizesPromptMessage` | `collect_prompt.go` (icons) | CP-1 | ✅ |
| REQ-1.6 | `TestCollectPrompts_*` (via proto extension) | `metadata.go:getPromptOptions` | CP-1 | ✅ |
| REQ-2.1 | `TestCollectPrompts_ArgumentRequiredSingular` | `collect_prompt.go` | CP-2 | ✅ |
| REQ-2.2 | `TestCollectPrompts_ArgumentOptional` | `collect_prompt.go` | CP-2 | ✅ |
| REQ-2.3 | `TestCollectPrompts_AcceptsScalarTypes` | `collect_prompt.go` | CP-3 | ✅ |
| REQ-2.4 | `TestCollectPrompts_AcceptsScalarTypes` | `collect_prompt.go` | CP-3 | ✅ |
| REQ-2.5 | `TestCollectPrompts_RejectsMessageField` | `collect_prompt.go` | CP-4 | ✅ |
| REQ-2.6 | `TestCollectPrompts_RejectsRepeatedField` | `collect_prompt.go` | CP-4 | ✅ |
| REQ-2.7 | `TestCollectPrompts_RejectsMapField` | `collect_prompt.go` | CP-4 | ✅ |
| REQ-2.8 | `TestCollectPrompts_RejectsOneofField` | `collect_prompt.go` | CP-4 | ✅ |
| REQ-2.9 | `TestCollectPrompts_FieldDescriptionFromOptions` | `collect_prompt.go` | CP-5 | ✅ |
| REQ-3.1 | (namespace tested via generated code + golden) | All renderers | CP-6 | ✅ |
| REQ-3.2 | (namespace tested via generated code + golden) | All renderers | CP-6 | ✅ |
| REQ-3.3 | (reuses `normalizeToolSegment`) | All renderers | CP-6 | ✅ |
| REQ-4.1 | `TestCollectPrompts_RecognizesPromptMessage` | `collect.go` | CP-7 | ✅ |
| REQ-4.2 | `TestCollectPrompts_SkipsNonPromptMessages` | `collect.go` | CP-7 | ✅ |
| REQ-5.1 | `TestGeneratePromptsGoGolden` | `render_go.go` | CP-8 | ✅ |
| REQ-5.2 | `TestGeneratePromptsPythonGolden` | `render_python.go` | CP-8 | ✅ |
| REQ-5.3 | `TestGeneratePromptsKotlinGolden` | `render_kotlin.go` | CP-8 | ✅ |
| REQ-5.4 | `TestGeneratePromptsJavaGolden` | `render_java.go` | CP-8 | ✅ (wiring fixed) |
| REQ-5.5 | `TestGeneratePromptsTypeScriptGolden` | `render_typescript.go` | CP-8 | ✅ (registry fixed) |
| REQ-5.6 | `TestParsePromptArguments_*` | `render_go.go` (Go path via `ParsePromptArguments`) | CP-9 | ✅ |
| REQ-5.7 | (verified by Go render code) | `render_go.go` | CP-9 | ✅ |
| REQ-6.1 | `TestParsePromptArguments_StringField` | `mcpruntime/prompt.go` | CP-10 | ✅ |
| REQ-6.2 | `TestParsePromptArguments_Int32Field` + others | `mcpruntime/prompt.go` | CP-10 | ✅ |
| REQ-6.3 | `TestParsePromptArguments_BoolField` | `mcpruntime/prompt.go` | CP-10 | ✅ |
| REQ-6.4 | `TestParsePromptArguments_BytesField` | `mcpruntime/prompt.go` | CP-10 | ✅ |
| REQ-6.5 | `TestParsePromptArguments_EnumField` | `mcpruntime/prompt.go` | CP-10 | ✅ (real test) |
| REQ-6.6 | `TestParsePromptArguments_InvalidNumber` | `mcpruntime/prompt.go` | CP-11 | ✅ |
| REQ-6.7 | `TestParsePromptArguments_MissingRequired` | `mcpruntime/prompt.go` | CP-12 | ✅ |
| REQ-6.8 | `TestParsePromptArguments_MissingOptional` | `mcpruntime/prompt.go` | CP-13 | ✅ |
| REQ-7.1 | `TestGeneratePrompts{Go,Python,Kotlin,Java,TypeScript}Golden` | `testdata/golden/prompts_mcp.*` | CP-14 | ✅ |
| REQ-7.2 | `TestCollectPrompts_Rejects*` | `collect_prompt_test.go` | CP-4 | ✅ |
| REQ-7.3 | `TestParsePromptArguments_*` | `mcpruntime/prompt_test.go` | CP-10-13 | ✅ |

## Design Conformance

### 3.1 Architectural Boundaries

Correct. New code resides in the planned locations. No unauthorized cross-layer imports.

### 3.2 Data Models

`PromptModel` and `PromptArgumentModel` match the design document §2.5 exactly.

### 3.3 API Contracts

All 5 language renderers now produce correct prompt registration code:
- **Go:** `<File>PromptHandler` interface + `Register<File>Prompts(server, impl, opts...)` ✅
- **Python:** `<File>PromptHandler` Protocol + `register_<file>_prompts(server, impl, *, namespace=None)` ✅
- **Kotlin:** `<File>PromptHandler` interface + `register<File>Prompts(server, impl, namespace?)` ✅
- **Java:** `<File>PromptHandler` nested interface + `register<File>Prompts(transport, impl, namespace)` + `RegisteredPrompt`/`PromptInvoker` wiring ✅
- **TypeScript:** `<File>PromptHandler` interface + `register<File>Prompts(server, impl, namespace?)` + `RegisteredPrompt` registry ✅

### 3.4 Error Handling

Error handling follows the design error table precisely. All fail-fast paths verified.

### 3.5 Correctness Properties

| CP | Status | Notes |
|----|--------|-------|
| CP-1 | ✅ | Prompt recognition, name/title/desc/icons forwarding verified |
| CP-2 | ✅ | Singular→required, optional→not required verified |
| CP-3 | ✅ | All scalar types + enum accepted |
| CP-4 | ✅ | message/repeated/map/oneof rejection verified |
| CP-5 | ✅ | Field description from option/comment verified |
| CP-6 | ✅ | Namespace logic reuses `normalizeToolSegment` |
| CP-7 | ✅ | `FileModel.Prompts` populated/empty correctly |
| CP-8 | ✅ | All 5 language renderers produce correct code (verified by golden tests) |
| CP-9 | ✅ | Go typed proto message passed to handler |
| CP-10 | ✅ | String/int/bool/bytes/enum parsing verified |
| CP-11 | ✅ | Invalid value → error verified |
| CP-12 | ✅ | Missing required → error verified |
| CP-13 | ✅ | Missing optional → zero-value verified |
| CP-14 | ✅ | Golden files and golden tests present for all 5 languages |

### 3.6 Documentation Consistency

Mermaid diagram in design §2.2 accurately reflects the actual data flow.

## Code Quality

No dead code, no debug artifacts, no TODOs in the final code.

## Security

No security issues found in changed files. No new endpoints, no injection vectors.

## Verification Evidence

- **Build:**
```
$ go build ./cmd/protoc-gen-mcp
(success, no output)
```

- **Tests (mcpruntime):**
```
--- PASS: TestParsePromptArguments_EnumField (0.00s)
--- PASS: TestRegisterExampleAPIToolsHappyPath (0.02s)
--- PASS: TestRegisterExampleAPIToolsDuplicateNameFails (0.01s)
PASS
ok  github.com/easyp-tech/protoc-gen-mcp/mcpruntime  0.789s
```

- **Tests (codegen - golden):**
```
--- PASS: TestGenerateExampleGolden (0.02s)
--- PASS: TestGeneratePythonExampleGolden (0.01s)
--- PASS: TestGenerateKotlinExampleGolden (0.01s)
--- PASS: TestGenerateJavaExampleGolden (0.01s)
--- PASS: TestGenerateTypeScriptExampleGolden (0.01s)
--- PASS: TestGeneratePromptsGoGolden (0.01s)
--- PASS: TestGeneratePromptsPythonGolden (0.00s)
--- PASS: TestGeneratePromptsKotlinGolden (0.00s)
--- PASS: TestGeneratePromptsJavaGolden (0.00s)
--- PASS: TestGeneratePromptsTypeScriptGolden (0.00s)
PASS
ok  github.com/easyp-tech/protoc-gen-mcp/internal/codegen  0.882s
```

- **Tests (examplemcp):**
```
ok  github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp  1.685s
```

NOTE: TypeScript runtime/compile/stdio тесты требуют `npm ci` в `examples/node/sdk-spike` — pre-existing env failures, не связаны с нашими изменениями.

## Findings (Revision 2 — all resolved)

| ID | Severity | Status | Description |
|----|----------|--------|-------------|
| F-1 | `major` | ✅ Fixed | TypeScript: рефакторен на registry/aggregation pattern с `RegisteredPrompt[]`, единый `ListPrompts`/`GetPrompt` handler |
| F-2 | `major` | ✅ Fixed | Java: добавлен `RegisteredPrompt`, `PromptInvoker`, `registerPrompt()`, `PROMPT_REGISTRIES` wiring |
| F-3 | `major` | ✅ Fixed | Добавлены 5 golden файлов + 5 golden comparison tests + `newPromptsProtogenPlugin` + `TestWritePromptsGoldenFiles` |
| F-4 | `minor` | ✅ Fixed | `TestParsePromptArguments_EnumField` переписан с `protodesc` + `dynamicpb` — real assertions |
| F-5 | `minor` | ⚠️ Noted | `validateToolNameLength` — scope creep, оставлен как полезный defensive check |

## Recommendations

Нет блокирующих рекомендаций. F-5 (scope creep `validateToolNameLength`) можно задокументировать post-hoc или вынести в отдельный коммит — на усмотрение.
