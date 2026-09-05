# Interagent

Приложение-оверлей для интервью и презентаций: слушает системный и микрофонный звук (STT),
захватывает скриншоты (OCR), отправляет текст в облачный OpenAI-совместимый LLM и показывает
подсказки в прозрачном окне поверх всех окон.

**Конфиденциальность по умолчанию.** Аудио всегда остаётся на устройстве (локальный STT — whisper.cpp).
ApiKey хранится зашифрованным (AES-256-GCM, мастер-ключ в Keychain). См. [ADR-001](docs/adr/001-local-vs-api-llm.md),
[ADR-004](docs/adr/004-cloud-llm.md), [ADR-005](docs/adr/005-inference-without-cgo.md).

## Как запустить

```
go generate ./...     # wails Generate
LLM_API_KEY=... wails dev    # dev-режим с облачным LLM
wails build           # production-сборка .app
```

Разработка требует: Go 1.25+, Node 22 + pnpm, macOS 14+. Аудио-тест-цикл и подпись см. в [AGENTS.md](AGENTS.md).

## Проверки

```
go test ./... && go vet ./... && go fmt ./...     # Go
cd frontend && pnpm lint && pnpm format:check && pnpm test
cd frontend && pnpm exec playwright test           # e2e
```
