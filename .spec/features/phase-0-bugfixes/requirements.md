# Phase 0: Tool Name Length Validation — Requirements

**Status:** Draft
**Author:** AI Agent
**Date:** 2026-06-09

## Обзор

Генератор `protoc-gen-mcp` не проверяет длину финального имени MCP tool. Длинные комбинации `namespace + "_" + methodName` могут превышать 64 символа — лимит Claude Desktop и ряда MCP клиентов. Генератор должен падать с понятной ошибкой на этапе кодогенерации если имя превышает лимит, чтобы пользователь исправил proto определение до компиляции.

## Требования

**REQ-1.1** WHEN генератор формирует финальное имя tool как `namespace + "_" + methodName` и длина результата ≤ 64 символа, the system SHALL принять имя без ошибки и передать его в renderer.

**REQ-1.2** WHEN генератор формирует финальное имя tool и длина результата > 64 символа, the system SHALL завершить генерацию с ошибкой, содержащей: полное имя tool, его текущую длину и максимально допустимую длину (64).

**REQ-1.3** WHEN namespace в proto `service` options пустой, the system SHALL формировать имя tool как `methodName` (без namespace prefix) и проверять длину только этого имени.

**REQ-1.4** WHEN имя tool содержит символы `.` (dot), the system SHALL сначала нормализовать их в `_` (underscore) и затем проверять длину нормализованного имени.

**REQ-1.5** WHEN генерация выполняется для любого из 5 поддерживаемых языков (`go`, `python`, `kotlin`, `java`, `typescript`), the system SHALL применять проверку длины единообразно, на уровне collector'а до вызова language-specific renderer'а.

**REQ-1.6** WHEN proto файл содержит несколько service'ов и только один из tool'ов превышает лимит, the system SHALL сообщить об ошибке для конкретного превышающего tool'а и не генерировать файлы.

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|----------|
| Test | `go test ./internal/codegen/... -count=1` | go.mod |
| Build | `go build ./cmd/protoc-gen-mcp` | go.mod |
| Lint | `easyp lint` | easyp.yaml |
