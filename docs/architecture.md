## 1. Principles

- **Clean / Hexagonal.** `adapter → port ← usecase ← bind ← frontend`. Layers point in the same direction.
- **Bind — only a dispatcher.** `bind_*.go` receives frontend calls, calls `usecase`, sends events. Knows nothing about adapters.
- **Adapter replacement without pain.** `internal/port/` — interfaces. `usecase` works through them.
- **Testability.** `usecase` is tested with adapter mocks. Adapters — integrationally.
- **ADR-001/005/011 mandatory.** STT — local only (whisper.cpp, cgo binding). LLM — local (Ollama, ADR-011) or cloud OpenAI-compatible, user's choice (ADR-004); apiKey encrypted (NFR-11).
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
- `internal/adapter/*` — port implementations: `audio/` (microphone + system sound + STT), `llm/` (local Ollama + openai-compatible cloud), `models/` (whisper model downloader), `screenshot/` (capture + OCR), `storage/` (SQLite), `window/` (overlay), `hotkeys/`, `system/` (macOS permissions).

**Dependency rule:** `adapter → port ← usecase ← bind ← frontend`. No circular dependencies. `port` does not depend on implementations. Changing a port (interface) — both `usecase` and all adapters change: done via ADR.

`LLM` is streaming-capable (ADR-011); the active agent's provider routes to the Ollama or OpenAI-compatible adapter via the `LLMFactory` wired in `app.go`. apiKey is encrypted at rest (ADR-004) via the `Crypto` port.

`Overlay`, `Hotkeys`, `Permissions` are introduced in ADR-006 / ADR-008 — they are needed right away for the stage-2 TDD tests (overlay behavior, click-through, shortcuts) and for handling macOS permissions. `LLM.Complete(input LLMInput, history, onToken)` — multimodal image input stays for stage 4.

> **Overlay** — the window is modeled as a port (ADR-006): `Show/Hide/Toggle/SetMode/GetMode`. Implementation — Wails + platform calls in `adapter/window`. Tested at the usecase layer with mocks.

> **Events** — async channel `usecase → frontend` through the `Events` port (`Emit`), Wails implementation in `adapter/events` (wrapper over `runtime.EventsEmit`, see ADR-010). Usecases emit events from `04-events.md` without knowing the recipient (frontend): a transport detail.
