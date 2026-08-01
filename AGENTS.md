# AGENTS.md

## Суть проекта

Interagent — приложение-оверлей, которое в реальном времени слушает интервью или презентацию (STT)
и захватывает скриншоты (OCR), отправляет текст в локальную или облачную LLM и показывает подски в прозрачном окне поверх всех окон.
Аудио никогда не покидает устройство. Скриншоты уходят в облако только при включённом облачном агенте и выбранном direct-image режиме (ADR-004, ADR-005); иначе — только текст транскрипций/OCR.

## Где документация

- `docs/00-documentation-map.md` — карта документов и порядок чтения.
- `docs/01-tz.md` — ТЗ, сущности, сценарии, словарь, ограничения, этапы.
- `docs/02-nfr.md` — нефункциональные требования.
- `docs/03-process.md` — процесс разработки (TDD, ревью, Definition of Done).
- `docs/04-events.md` — рантайм-события backend → frontend (типы и bind-методы живут в коде: `internal/port/types.go` ↔ `frontend/src/common/types/api.types.ts`).
- `docs/05-architecture.md` — слои, зависимости, пайплайн, окно, ошибки, хранилище.
- `docs/06-bind-contracts.md` — контракт bind-методов frontend ↔ backend.
- `docs/adr/` — архитектурные решения (ADR-001…008).

## Ключевые правила для агентов

1. **Тесты — before код.** Никакой реализации без зелёных тестов и ревью.
2. **ADR-001/005 обязательны:** STT — только локально (whisper.cpp, единственное cgo-место). LLM — локальный (llama.go, pure Go) или облачный OpenAI-совместимый по выбору пользователя (см. ADR-004); apiKey хранится зашифрованным (AES-256-GCM, мастер-ключ в Keychain).
3. **Изменение контракта или архитектуры требует ADR** (см. `docs/adr/README.md`).
4. **Чистые зависимости:** `adapter → port ← usecase ← bind ← frontend`. Ни `usecase`, ни `port` не зависят от реализаций адаптеров.
5. **Markdown форматируется только через Prettier.** После правки любого `.md` (docs/, AGENTS.md, README.md и т.д.) — прогнать `pnpm docs:format:check`; для автоформатирования — `pnpm docs:format`.

## Как запустить проверки

Соответствует GitHub Actions (`.github/workflows/test.yml`):

```
pnpm install            # pnpm workspace (корень репо): все пакеты + git-хуки
go test ./...           # Go backend (stdlib testing)
go vet ./...           # статический анализ
go fmt ./...           # formatting
pnpm docs:format:check # prettier для .md (docs/, AGENTS.md, README.md)
cd frontend && pnpm lint
cd frontend && pnpm format:check
cd frontend && pnpm test
cd frontend && pnpm exec playwright test   # e2e
```

Сборка: `wails build`.
