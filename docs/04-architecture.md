# Architecture

**Дата:** 2026-07-30
**Статус:** Утверждён

---

## Принципы

- **Clean/Hexagonal** — port / usecase / adapter. Bind — слой доставки (Wails handler).
- **Bind — только диспетчер.** `bind_session.go` вызывает `usecase.Session`, ничего не знает про адаптеры.
- **Замена адаптера без боли.** `internal/port/` — интерфейсы. `usecase` работает через них.
- **Тестируемость.** `usecase` тестируется с моками адаптеров. Адаптеры — интеграционно.
- **Feature-first фронтенд.** Каждая фича в своей папке `features/<name>/`.

---

## Диаграмма слоёв

```
  ┌──────────────────────────────────────────────────────────┐
  │  frontend/ (Vue 3 + TS)                                  │
  │  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
  │  │ overlay │ │  audio   │ │screenshot│ │ settings │    │
  │  │ session │ │          │ │          │ │          │    │
  │  └────┬────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘    │
  │       │           │            │            │          │
  │       └───────────┴────────────┴────────────┘          │
  │                      │ wailsjs (Events + Bind)         │
  └──────────────────────┼─────────────────────────────────┘
                         │
  ┌──────────────────────┼─────────────────────────────────┐
  │  backend/ (Go)       │                                  │
  │  ┌───────────────────┴──────────────┐                   │
  │  │  bind.go / bind_*.go            │ ← слой доставки   │
  │  │  (принимает вызовы фронта,      │                   │
  │  │   вызывает usecase, шлёт events)│                   │
  │  └───────────────────┬──────────────┘                   │
  │                      │                                  │
  │  ┌───────────────────┴──────────────┐                   │
  │  │  internal/usecase                │ ← бизнес-логика  │
  │  │  (зависит только от port)        │                   │
  │  └───────────────────┬──────────────┘                   │
  │                      │                                  │
  │  ┌───────────────────┴──────────────┐                   │
  │  │  internal/port                   │ ← интерфейсы     │
  │  │  + типы данных                   │                   │
  │  └───────────────────┬──────────────┘                   │
  │                      │                                  │
  │  ┌───────────────────┴──────────────┐                   │
  │  │  internal/adapter                │ ← реализации     │
  │  │  audio/  screenshot/  llm/      │                   │
  │  │  storage/                        │                   │
  │  └──────────────────────────────────┘                   │
  └──────────────────────────────────────────────────────────┘
```

---

## Структура директорий

```
interagent/
├── main.go                        # Wails.Run (точка входа)
├── go.mod
├── wails.json
│
├── bind.go                        # App struct, startup, Version, Quit
├── bind_session.go                # NewSession, GetSession, ClearSession
├── bind_audio.go                  # StartListening, StopListening, ...
├── bind_screenshot.go             # CaptureFullScreen, CaptureRegion, ...
├── bind_llm.go                    # SendText, CancelResponse
├── bind_agent.go                  # GetAgents, SetActiveAgent, ...
├── bind_settings.go               # GetSettings, SaveSettings, ...
│
├── internal/
│   ├── usecase/
│   │   ├── session.go             # Бизнес-логика сессий
│   │   ├── audio.go               # Микрофон → STT → текст
│   │   ├── screenshot.go          # Скриншот → OCR → текст
│   │   ├── llm.go                 # Текст → LLM → ответ
│   │   ├── agent.go               # Управление агентами
│   │   └── settings.go            # Управление настройками
│   │
│   ├── adapter/
│   │   ├── audio/
│   │   │   ├── recorder.go        # Захват микрофона (macOS AVFoundation)
│   │   │   └── whisper.go         # Whisper STT
│   │   ├── screenshot/
│   │   │   ├── capturer.go        # Захват экрана (CGDisplay)
│   │   │   └── ocr.go             # OCR (Tesseract / Apple Vision)
│   │   ├── llm/
│   │   │   └── engine.go          # llama.cpp инференс
│   │   └── storage/
│   │       └── store.go           # JSON / SQLite
│   │
│   └── port/
│       ├── types.go               # Session, Message, AgentConfig, AppSettings, Shortcut, AudioDevice
│       ├── audio.go               # AudioInput, STT интерфейсы
│       ├── screenshot.go          # ScreenCapture, OCR интерфейсы
│       ├── llm.go                 # LLM интерфейс
│       └── storage.go             # Storage интерфейс
│
├── frontend/
│   ├── main.ts
│   ├── App.vue                    # Рутовый компонент
│   ├── style.css
│   │
│   ├── types/
│   │   └── index.ts               # Все TS-типы (зеркало internal/port/types.go)
│   │
│   ├── utils/
│   │   └── wails.ts               # Обёртки над wailsjs/go/main/App
│   │
│   └── features/
│       ├── overlay/
│       │   ├── OverlayWindow.vue
│       │   ├── TranscriptionBar.vue
│       │   └── AnswerBox.vue
│       ├── session/
│       │   ├── ChatHistory.vue
│       │   └── MessageItem.vue
│       ├── audio/
│       │   ├── useAudio.ts
│       │   └── MicButton.vue
│       ├── screenshot/
│       │   ├── useScreenshot.ts
│       │   └── ScreenshotButton.vue
│       └── settings/
│           ├── useSettings.ts
│           ├── SettingsPanel.vue
│           ├── AgentManager.vue
│           └── ShortcutEditor.vue
│
├── docs/
└── build/
```

---

## Зависимости

| Слой | Импортирует |
|------|-------------|
| `bind.go / bind_*.go` | `internal/usecase`, `internal/port` |
| `internal/usecase` | `internal/port` (интерфейсы + типы) |
| `internal/adapter/*` | `internal/port`, внешние библиотеки |
| `internal/port` | (чистые интерфейсы + типы, без внешних зависимостей) |
| `frontend/features/*` | `frontend/types`, `wailsjs/go/main/App`, Wails runtime |

Все зависимости идут в одном направлении: `adapter → port ← usecase ← bind ← фронт`. Кольцевых зависимостей нет.

---

## Data flow: Аудио → STT → LLM → ответ

```
1. Фронт: StartListening() → бэкенд
2. Бэкенд: захват микрофона, стриминг PCM в Whisper
3. Бэкенд → событие transcription:partial (промежуточный текст)
4. Бэкенд → событие transcription:done (финальный текст)
5. Фронт: SendText(текст) → бэкенд
6. Бэкенд: LLM.Complete(prompt + история) → ответ
7. Бэкенд → событие llm:response
8. Фронт: рендер ответа в оверлее, добавление в историю
```

## Data flow: Скриншот → OCR → LLM → ответ

```
1. Фронт: CaptureFullScreen() / CaptureRegion() → бэкенд
2. Бэкенд: захват изображения (CGDisplay)
3. Бэкенд → событие screenshot:captured (preview)
4. Бэкенд: OCR.ExtractText(изображение) → текст
5. Бэкенд → событие ocr:done
6. Фронт: SendText(текст со скрина) → бэкенд
7. Бэкенд: LLM.Complete(prompt + история) → ответ
8. Бэкенд → событие llm:response
9. Фронт: рендер ответа
```
