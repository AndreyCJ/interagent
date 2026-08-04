# Events (Backend → Frontend)

**Date:** 2026-08-01
**Status:** Approved

---

## Rules

1. **Events are the only async channel** backend → frontend. Bind methods returning `void` answer with an event.
2. **Data types and bind methods — in code, not here.** Source of truth for types:
   - Go: `internal/port/types.go`
   - TypeScript: `frontend/src/common/types/api.types.ts`
   - JS bindings are generated: `wails generate` → `frontend/wailsjs/`
3. **Event names and payloads — only here.** Wails events are strings with JSON data without type checking, so their contract is documented manually.
4. **Changing events — via PR + review** (see DoD in `03-process.md`).
5. **STT — local (whisper.cpp). LLM — local (Ollama, ADR-011) or cloud OpenAI-compatible, user's choice** (ADR-001, ADR-004).

---

## Session

| Event             | Data      | When                |
| ----------------- | --------- | ------------------- |
| `session:created` | `Session` | New session created |

## Audio / STT

| Event                   | Data                                                     | When                                                                                |
| ----------------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `audio:level`           | `{ level: number }`                                      | Microphone level (throttled)                                                        |
| `transcription:partial` | `{ text: string }`                                       | Partial STT text                                                                    |
| `transcription:done`    | `{ text: string, confidence: number, language: string }` | Final STT text (`language` = whisper-detected code, e.g. `"en"`, `""` when unknown) |

## Models

| Event                     | Data                                                 | When                                   |
| ------------------------- | ---------------------------------------------------- | -------------------------------------- |
| `model:download-progress` | `{ model: string, received: number, total: number }` | Whisper model download progress        |
| `model:downloaded`        | `{ model: string }`                                  | Model downloaded and checksum-verified |

## Screenshot / OCR

| Event                 | Data                 | When                       |
| --------------------- | -------------------- | -------------------------- |
| `screenshot:captured` | `{ base64: string }` | Screenshot ready (preview) |
| `screenshot:error`    | `{ error: string }`  | Capture error              |
| `ocr:done`            | `{ text: string }`   | OCR complete               |

## LLM

| Event           | Data                | When                                      |
| --------------- | ------------------- | ----------------------------------------- |
| `llm:started`   | `{}`                | Generation started ("typing" indicator)   |
| `llm:partial`   | `{ text: string }`  | Streaming token (cumulative answer text)  |
| `llm:response`  | `{ text: string }`  | Full LLM response                         |
| `llm:error`     | `{ error: string }` | LLM error                                 |
| `llm:cancelled` | `{}`                | Generation cancelled (new input, ADR-007) |

## Overlay

| Event          | Data                                         | When                          |
| -------------- | -------------------------------------------- | ----------------------------- |
| `overlay:mode` | `{ mode: 'click-through' \| 'interactive' }` | Window mode changed (ADR-006) |

## Settings / agents

| Event              | Data             | When                 |
| ------------------ | ---------------- | -------------------- |
| `agent:changed`    | `{ id: string }` | Active agent changed |
| `settings:updated` | `AppSettings`    | Settings saved       |

## Common

| Event            | Data                                       | When                              |
| ---------------- | ------------------------------------------ | --------------------------------- |
| `app:error`      | `{ stage: string, error: string }`         | Any error                         |
| `app:permission` | `{ permission: string, granted: boolean }` | macOS permission status (ADR-008) |
