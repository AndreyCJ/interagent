# Architecture

**Date:** 2026-07-30
**Status:** Approved

---

## 1. Principles

- **Clean / Hexagonal.** `adapter → port ← usecase ← bind ← frontend`. Layers point in the same direction.
- **Bind — only a dispatcher.** `bind_*.go` receives frontend calls, calls `usecase`, sends events. Knows nothing about adapters.
- **Adapter replacement without pain.** `internal/port/` — interfaces. `usecase` works through them.
- **Testability.** `usecase` is tested with adapter mocks. Adapters — integrationally.
- **ADR-001/005 mandatory.** STT — local only (whisper.cpp, cgo binding). LLM — local (llama.go, pure Go) or cloud OpenAI-compatible, user's choice (ADR-004); apiKey encrypted (NFR-11).
- **Feature-first frontend.** Each feature lives in its own folder `frontend/features/<name>/`.

---

## 2. Layers and dependencies

The stack is three layers in one dependency direction (`adapter → port ← usecase ← bind ← frontend`):

**1. Frontend (`frontend/`)**

The frontend architecture follows the FEOD methodology (Fractal Entity Oriented Design).

- `app/` — the app entity that describes everything needed to launch the app and configure it. This is where the key things needed only to run the app itself live.
- `features/<name>/` — a feature is a unique, reusable module. Modules must be isolated from each other as much as possible. Access to a module's internals is possible only through its public API. Inside, modules can have their own types, stores, composables, etc.
- `common/` — the level defining entities for shared reuse. These are entities not tied to specific business logic that can be used anywhere in the project. Also standalone entities that are hard to attribute to a specific module.

Data types: `frontend/src/common/types/api.types.ts` (mirror of the Go types).

Event contract backend → frontend: `docs/04-events.md`.

**Dependency rules:**

- app — cannot be imported and is the entry point of the app
- common — can be imported at any level
- features — can be imported by app

Frontend dependency chain — `common ➜ features ➜ app`

**2. Bridge: Wails (`frontend/wailsjs/`)** — generated bindings (`wails generate`). The frontend calls Go methods synchronously, results of async operations arrive as events.

**3. Backend (Go):**

- `bind.go / bind_*.go` — delivery layer. Receives frontend calls, delegates to `usecase`, sends events. Knows nothing about adapters.
- `internal/usecase` — business logic. Depends only on `internal/port`.
- `internal/port` — interfaces and data types, no external dependencies.
- `internal/adapter/*` — port implementations: `audio/` (microphone + system sound + STT), `llm/` (local llama.go + openai-compatible cloud), `screenshot/` (capture + OCR), `storage/` (SQLite), `window/` (overlay), `hotkeys/`, `system/` (macOS permissions).

**Dependency rule:** `adapter → port ← usecase ← bind ← frontend`. No circular dependencies. `port` does not depend on implementations. Changing a port (interface) — both `usecase` and all adapters change: done via ADR.

### Ports (interfaces)

| Interface         | Methods                                                 | Defined in                     |
| ----------------- | ------------------------------------------------------- | ------------------------------ |
| `AudioInput`      | Start, Stop, Devices, SetDevice                         | `internal/port/audio.go`       |
| `STT`             | Transcribe(audioData) → (text, confidence)              | `internal/port/audio.go`       |
| `ScreenCapture`   | CaptureFull, CaptureRegion                              | `internal/port/screenshot.go`  |
| `OCR`             | ExtractText(image) → string                             | `internal/port/screenshot.go`  |
| `LLM`             | Complete(input, history) → string, Cancel               | `internal/port/llm.go`         |
| `SessionStorage`  | CreateSession, GetSession, UpdateSession, DeleteSession | `internal/port/storage.go`     |
| `AgentStorage`    | GetAgents, SaveAgent, DeleteAgent                       | `internal/port/storage.go`     |
| `SettingsStorage` | GetSettings, SaveSettings                               | `internal/port/storage.go`     |
| `Overlay`         | Show, Hide, Toggle, SetMode, GetMode                    | `internal/port/overlay.go`     |
| `Hotkeys`         | Register, Unregister                                    | `internal/port/hotkeys.go`     |
| `Permissions`     | Status, Request, OpenSettings                           | `internal/port/permissions.go` |
| `Events`          | Emit(name, payload)                                     | `internal/port/events.go`      |

`Overlay`, `Hotkeys`, `Permissions` are introduced in ADR-006 / ADR-008 — they are needed right away for the stage-2 TDD tests (overlay behavior, click-through, shortcuts) and for handling macOS permissions. `LLM.Complete` accepts `input { text, image? }` — multimodal (ADR-005).

> **Overlay** — the window is modeled as a port (ADR-006): `Show/Hide/Toggle/SetMode/GetMode`. Implementation — Wails + platform calls in `adapter/window`. Tested at the usecase layer with mocks.

> **Events** — async channel `usecase → frontend` through the `Events` port (`Emit`), Wails implementation in `adapter/events` (wrapper over `runtime.EventsEmit`, see ADR-010). Usecases emit events from `04-events.md` without knowing the recipient (frontend): a transport detail.

> **Sources of truth.** Data types and bind methods live in code: Go — `internal/port/types.go` (types overlapping with the frontend are mirrored there: Message, Session, AgentConfig, AppSettings, Shortcut, AudioDevice; modes and events — constants/interfaces in `internal/port/{overlay,hotkeys,permissions,events}.go`), TS — `frontend/src/common/types/api.types.ts`, bindings generated by `wails generate`. Only the runtime events contract is documented: `docs/04-events.md`.

---

## 3. Common pipeline (one pattern for all inputs)

All scenarios reduce to a single chain: **Input → Enrich → LLM → Render**.

| Step      | What                                                | Who                                               | Where                                                        |
| --------- | --------------------------------------------------- | ------------------------------------------------- | ------------------------------------------------------------ |
| 1. Input  | Input arrival (text, STT, screenshot)               | frontend / bind                                   | Stage 2 / 3 / 4                                              |
| 2. Enrich | (optional) STT / OCR convert media → text           | adapter: audio/stt, screenshot/ocr                |                                                              |
| 3. LLM    | `LLM.Complete(input, history)` → string             | usecase → port.LLM → adapter:llm (local: llama.go | cloud: openai-compatible) — choice by `AgentConfig.provider` |
| 4. Render | Output the answer to the overlay + write to history | frontend → bind (event `llm:response`)            |                                                              |
| Error     | `app:error { stage, error }` + local log            | any layer                                         | NFR-06, NFR-09                                               |

**Manual input (Stage 2).** `frontend → SendText(text) → bind → usecase LLM.Complete → llm:started / llm:response` (through the `Events` port). The usecase appends to the current session history (`user` + `assistant`); `llm:error` — without crashing (NFR-06).

**Audio (Stage 3).** `StartListening → AudioInput (microphone / system sound SCK) → STT (whisper.cpp, endpoint detection) → transcription:done → SendText → LLM → llm:response`. On new input during generation — cancel and generate on the fresh input (ADR-007).

**Screenshot (Stage 4) — two paths:**

- **OCR:** `CaptureFullScreen/Region → ScreenCapture → OCR → ocr:done → SendText → LLM → llm:response` (text).
- **Multimodal (direct-image):** `CaptureFullScreen/Region → SendImage({base64, format}) → LLM.Complete(input{text, image}) → llm:response` (screenshot as-is, ADR-005).

Inference: **STT local** (whisper.cpp via cgo binding, ADR-005); **LLM — hybrid** (llama.go locally or OpenAI-compatible cloud, user's choice); OCR — Apple Vision (see ADR-001, ADR-002, ADR-003, ADR-004, ADR-005).

---

## 4. Window (overlay)

This is the core of the product. Modeled as the `Overlay` port (ADR-006), implemented in `adapter/window` + `bind`/`frontend`:

- **always-on-top** — the window is above all windows (including fullscreen).
- **click-through by default** — mouse and keyboard pass through the window (NFR-04).
- **interactive mode** — toggled by a global shortcut (default `Cmd+Shift+Space`), manual input and settings are available in it. The mode is reflected by the `overlay:mode` event (ADR-006).
- **single window** — multi-monitor configurations are out of scope for v1 (01-tz §7).
- **transparency / themes** — `AppSettings.theme ∈ {dark, light, transparent}`.

---

## 5. Errors

- **Policy.** An adapter error (STT/LLM/OCR/Storage timeout, panic, invalid response, HTTP error of the cloud LLM) **does not bring down the app**. The usecase returns an error → bind sends `app:error { stage, error }` to the frontend.
- **Cloud.** Timeout/4xx/5xx of the cloud LLM (NFR-06) → fallback to the local LLM if available; otherwise — a message in the overlay.
- **Behavior.** On any error the frontend shows a clear message in the overlay and a retry option (NFR-06).
- **Logging.** All errors → local log `timestamp, stage, context` (NFR-09). The user can export the log. No telemetry (opt-in — out of scope for v1).

---

## 6. Storage

- All entity data (session history, agents, settings, encrypted apiKey) — locally, on the device. Not synced anywhere.
- Storage technology — SQLite (pure-Go, no cgo), see ADR-003.
- `apiKey` is stored encrypted (AES-256-GCM, NFR-11); the master key — in macOS Keychain (ADR-004).

---

## 7. Out of scope (v1)

- Cloud STT (cloud whisper) — not v1 (STT local only, ADR-001).
- Native cloud LLM providers (Anthropic, Gemini) — not v1; only OpenAI-compatible (ADR-004).
- Windows / Linux (first version — macOS only).
- Multi-monitor configurations.
- Telemetry (optional, opt-in).
- Auto-update (NFR-10) — implemented at the end (Stage 5), does not affect the architecture for now.
