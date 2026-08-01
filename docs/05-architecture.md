# Architecture

**Дата:** 2026-07-30
**Статус:** Утверждён

---

## 1. Принципы

- **Clean / Hexagonal.** `adapter → port ← usecase ← bind ← frontend`. Слои направлены одинаково.
- **Bind — только диспетчер.** `bind_*.go` принимает вызовы фронта, вызывает `usecase`, шлёт events. Ничего не знает про адаптеры.
- **Замена адаптера без боли.** `internal/port/` — интерфейсы. `usecase` работает через них.
- **Тестируемость.** `usecase` тестируется с моками адаптеров. Адаптеры — интеграционно.
- **ADR-001/005 обязательны.** STT — только локальный (whisper.cpp, cgo-биндинг). LLM — локальный (llama.go, pure Go) или облачный OpenAI-compatible по выбору (ADR-004); apiKey зашифрован (NFR-11).
- **Feature-first фронтенд.** Каждая фича в своей папке `frontend/features/<name>/`.

---

## 2. Слои и зависимости

Стек — три слоя в одном направлении зависимостей (`adapter → port ← usecase ← bind ← frontend`):

**1. Frontend (`frontend/`)**

Архитектура фронтенда организована по методологии FEOD (Fractal Entity Oriented Design).

- `app/` - сущность приложения, которая описывает всё, что необходимо для запуска приложения и его настройки. Здесь находятся ключевые вещи, которые нужны только для запуска самого приложения.
- `features/<name>/` - фича является уникальным, переиспользуемым модулем. Модули должны быть изолированы друг от друга настолько, насколько это возможно. Доступ к внутренностям модуля возможен только через публичный API. Внутри модули могут иметь свои типы, сторы, композаблы и тд.
- `common/` - уровень, определяющий сущности для общего переиспользования. Это сущности, которые не привязаны к конкретной бизнес-логике и могут использоваться в любом месте проекта. А также одиночные сущности которые сложно причислить к какому-то конкретному модулю.

Типы данных: `frontend/src/common/types/api.types.ts` (зеркало Go-типов).

Контракт событий backend → frontend: `docs/04-events.md`.

**Правило зависимостей:**

- app - не может быть импортирован и является входной точкой в приложение
- common - может быть импортирован на любом уровне
- features - могут быть импортированы app

Цепочка зависимостей фронтенда - `common ➜ features ➜ app`

**2. Bridge: Wails (`frontend/wailsjs/`)** — сгенерированные биндинги (`wails generate`). Frontend вызывает Go-методы синхронно, результаты асинхронных операций приходят событиями.

**3. Backend (Go):**

- `bind.go / bind_*.go` — слой доставки. Принимает вызовы фронта, делегирует в `usecase`, шлёт события. Про адаптеры не знает.
- `internal/usecase` — бизнес-логика. Зависит только от `internal/port`.
- `internal/port` — интерфейсы и типы данных, без внешних зависимостей.
- `internal/adapter/*` — реализации портов: `audio/` (микрофон + системный звук + STT), `llm/` (локальный llama.go + openai-compatible облако), `screenshot/` (захват + OCR), `storage/` (SQLite), `window/` (overlay), `hotkeys/`, `system/` (разрешения macOS).

**Правило зависимостей:** `adapter → port ← usecase ← bind ← frontend`. Кольцевых зависимостей нет. `port` не зависит от реализаций. Изменить порт (интерфейс) — меняется и `usecase`, и все адаптеры: делается через ADR.

### Порты (интерфейсы)

| Интерфейс       | Методы                                     | Где определён                  |
| --------------- | ------------------------------------------ | ------------------------------ |
| `AudioInput`    | Start, Stop, Devices, SetDevice            | `internal/port/audio.go`       |
| `STT`           | Transcribe(audioData) → (text, confidence) | `internal/port/audio.go`       |
| `ScreenCapture` | CaptureFull, CaptureRegion                 | `internal/port/screenshot.go`  |
| `OCR`           | ExtractText(image) → string                | `internal/port/screenshot.go`  |
| `LLM`           | Complete(input, history) → string, Cancel  | `internal/port/llm.go`         |
| `Storage`       | CRUD для Session, Agents, Settings         | `internal/port/storage.go`     |
| `Overlay`       | Show, Hide, Toggle, SetMode, GetMode       | `internal/port/overlay.go`     |
| `Hotkeys`       | Register, Unregister                       | `internal/port/hotkeys.go`     |
| `Permissions`   | Status, Request, OpenSettings              | `internal/port/permissions.go` |

`Overlay`, `Hotkeys`, `Permissions` заведены в ADR-006 / ADR-008 — они нужны сразу для TDD-тестов этапа 2 (поведение оверлея, click-through, шорткаты) и для обработки macOS-разрешений. `LLM.Complete` принимает `input { text, image? }` — multimodal (ADR-005).

> **Overlay** — окно моделируется как порт (ADR-006): `Show/Hide/Toggle/SetMode/GetMode`. Реализация — Wails + платформенные вызовы в `adapter/window`. Тестируется usecase-слой с моками.

> **Источники правды.** Типы данных и bind-методы живут в коде: Go — `internal/port/types.go` (там же зеркалятся типы, пересекающиеся с фронтендом: Message, Session, AgentConfig, AppSettings, Shortcut, AudioDevice), TS — `frontend/src/common/types/api.types.ts`, биндинги генерируются `wails generate`. Документально описан только контракт рантайм-событий: `docs/04-events.md`.

---

## 3. Общий пайплайн (один паттерн для всех входов)

Все сценарии сводятся к одной цепочке: **Input → Enrich → LLM → Render**.

| Шаг       | Что                                               | Кто                                               | Где                                                         |
| --------- | ------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------- | --- |
| 1. Input  | Поступление инпута (текст, STT, скриншот)         | frontend / bind                                   | Этап 2 / 3 / 4                                              |
| 2. Enrich | (опционально) STT / OCR преобразуют media → текст | adapter: audio/stt, screenshot/ocr                |                                                             |
| 3. LLM    | `LLM.Complete(input, history)` → string           | usecase → port.LLM → adapter:llm (local: llama.go | cloud: openai-compatible) — выбор по `AgentConfig.provider` |     |
| 4. Render | Вывод ответа в оверлей + запись в историю         | frontend → bind (event `llm:response`)            |                                                             |
| Ошибка    | `app:error { stage, error }` + локальный лог      | любой слой                                        | NFR-06, NFR-09                                              |

**Ручной ввод (Этап 2).** `frontend → SendText(text) → bind → usecase LLM.Complete → llm:response`. История сессии пополняется.

**Аудио (Этап 3).** `StartListening → AudioInput (микрофон / системный звук SCK) → STT (whisper.cpp, endpoint detection) → transcription:done → SendText → LLM → llm:response`. При новом вводе во время генерации — cancel и генерация на свежий ввод (ADR-007).

**Скриншот (Этап 4) — два пути:**

- **OCR:** `CaptureFullScreen/Region → ScreenCapture → OCR → ocr:done → SendText → LLM → llm:response` (текст).
- **Multimodal (direct-image):** `CaptureFullScreen/Region → SendImage({base64, format}) → LLM.Complete(input{text, image}) → llm:response` (скриншот как есть, ADR-005).

Инференс: **STT локальный** (whisper.cpp через cgo-биндинг, ADR-005); **LLM — гибрид** (llama.go локально или OpenAI-compatible облако по выбору пользователя); OCR — Apple Vision (см. ADR-001, ADR-002, ADR-003, ADR-004, ADR-005).

---

## 4. Окно (overlay)

Это ядро продукта. Моделируется портом `Overlay` (ADR-006), реализуется в `adapter/window` + `bind`/`frontend`:

- **always-on-top** — окно поверх всех окон (включая fullscreen).
- **click-through по умолчанию** — мышь и клавиатура проходят сквозь окно (NFR-04).
- **кликабельный режим** — переключается глобальным шорткатом (по умолчанию `Cmd+Shift+Space`), в нём доступен ручной ввод и настройки. Режим отражается событием `overlay:mode` (ADR-006).
- **одно окно** — мультимониторные конфигурации out of scope для v1 (01-tz §7).
- **прозрачность / темы** — `AppSettings.theme ∈ {dark, light, transparent}`.

---

## 5. Ошибки

- **Политика.** Ошибка адаптера (STT/LLM/OCR/Storage timeout, panic, невалидный ответ, HTTP-ошибка облачного LLM) **не роняет приложение**. usecase возвращает ошибку → bind шлёт `app:error { stage, error }` фронту.
- **Облако.** Таймаут/4xx/5xx облачного LLM (NFR-06) → fallback на локальный LLM, если он доступен; иначе — сообщение в оверлее.
- **Поведение.** На любой ошибке фронтенд показывает в оверлее понятное сообщение и возможность повторить (NFR-06).
- **Логирование.** Все ошибки → локальный лог `timestamp, stage, context` (NFR-09). Пользователь может экспортировать лог. Без телеметрии (opt-in — out of scope v1).

---

## 6. Хранилище

- Вся сущность (история сессий, агенты, настройки, зашифрованные apiKey) — локально, на устройстве. Никуда не синхронится.
- Технология хранилища — SQLite (pure-Go, без cgo), см. ADR-003.
- `apiKey` хранится зашифрованным (AES-256-GCM, NFR-11); мастер-ключ — в macOS Keychain (ADR-004).

---

## 7. Out of scope (v1)

- Облачный STT (облачный whisper) — не v1 (STT только локальный, ADR-001).
- Нативные облачные LLM-провайдеры (Anthropic, Gemini) — не v1; только OpenAI-compatible (ADR-004).
- Windows / Linux (первая версия — только macOS).
- Мультимониторные конфигурации.
- Телеметрия (опционально, opt-in).
- Автообновление (NFR-10) — реализуется в конце (Этап 5), пока не влияет на архитектуру.
