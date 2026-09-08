# AGENTS.md

## Суть проекта

Interagent — приложение-оверлей, которое в реальном времени слушает интервью или презентацию (STT)
и захватывает скриншоты (OCR), отправляет текст в локальную или облачную LLM и показывает подсказки в прозрачном окне поверх всех окон.
Аудио никогда не покидает устройство. Скриншоты уходят в облако только при включённом облачном агенте и выбранном direct-image режиме (ADR-004, ADR-005); иначе — только текст транскрипций/OCR.

## Контракты живут в коде

Документация намеренно минимальна — источником истины является код (`docs/adr/` хранит только решения):

- Рантайм-события backend → frontend и типы: `internal/port/types.go`, `internal/port/events.go` ↔ `frontend/src/common/types/api.types.ts`.
- Контракт bind-методов: `bind_*.go` (тонкие диспетчеры без логики).
- Слои/пайплайн/ошибки/хранилище — по структуре `internal/{adapter,port,usecase}` и `app.go`.

## Ключевые правила для агентов

1. **Тесты — before код.** Никакой реализации без зелёных тестов и ревью (TDD, Definition of Done).
2. **STT — только локально** (whisper.cpp, ADR-001/005/011). **cgo — два замороженных исключения**
   (ADR-005 + amendment, ADR-013): whisper.cpp (STT) и Linux-аудио
   (`internal/adapter/audio/capture_{pulse,pipewire}_linux.go`, libpulse-simple + PipeWire-мост;
   портальный D-Bus-флоу — чистый Go, `internal/adapter/portal`); остальной код — чистый Go.
   LLM — облачный OpenAI-совместимый (адаптер `internal/adapter/llm/openai`, ADR-004); в dev ключ
   задаётся через `LLM_API_KEY`/`LLM_BASE_URL`/`LLM_MODEL` (env-оверрайд в `app.go`), в прод —
   через агента с зашифрованным apiKey (AES-256-GCM, мастер-ключ в Keychain).
3. **Изменение контракта или архитектуры требует ADR** (см. `docs/adr/README.md`).
4. **Чистые зависимости:** `adapter → port ← usecase ← bind ← frontend`. Ни `usecase`, ни `port` не зависят от реализаций адаптеров.
5. **Мик (говорящий пользователь)** — только история (`user` role), без автозапроса к LLM. Автоответ
   запускает только системный звук (интервьюер), ADR-007 cancel-on-new-input.

## Как запустить проверки

Соответствует GitHub Actions (`.github/workflows/test.yml`):

```
pnpm install               # pnpm workspace (корень репо): все пакеты + git-хуки
pnpm --dir frontend build  # go:embed требует frontend/src/app/dist (перед go-проверками)
go test ./...              # Go backend (stdlib testing)
go vet ./...               # статический анализ
go fmt ./...               # formatting
pnpm docs:format:check     # prettier для .md (AGENTS.md, README.md, docs/adr/)
cd frontend && pnpm lint
cd frontend && pnpm format:check
cd frontend && pnpm test
cd frontend && pnpm exec playwright test   # e2e
```

Сборка: `wails build`. Dev-запуск с облачным LLM: `LLM_API_KEY=... wails dev`.

## macOS: аудио-дев-цикл и подпись

- ScreenCaptureKit (системный звук, скриншоты/OCR, ADR-012) на Sequoia/Tahoe отклоняет неподписанные / ad-hoc / self-signed бинарники (`SCError 1003`), а TCC-гранты привязаны к подписи кода.
- Аудио-тест-цикл: `./scripts/dev-audio.sh` — собирает `.app`, подписывает стабильной identity (авто-выбор: `$IA_DEV_SIGN_IDENTITY` → `Apple Development:` → self-signed), открывает bundle. `wails dev` для аудио не подходит (запускает неподписанный bare-бинарник).
- Гранты Screen & System Audio Recording + Microphone выдаются один раз для `interagent.app` (bundle id `com.wails.interagent`) и держатся между пересборками только при стабильной подписи. Если после переподписи грант «слетел» — `tccutil reset ScreenCapture com.wails.interagent` (+ `Microphone`) и выдать заново.
