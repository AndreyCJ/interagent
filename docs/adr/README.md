# Architectural Decision Records (ADR)

## Index

| ADR     | Title                                             | Status   | Date       | Summary                                                                                                                                    |
| ------- | ------------------------------------------------- | -------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| ADR-001 | Local STT / hybrid LLM                            | Accepted | 2026-07-29 | STT — whisper.cpp (local). LLM — local (llama.cpp) or cloud (OpenAI-compatible), user's choice. See NFR-02, NFR-11.                        |
| ADR-002 | OCR: Apple Vision vs Tesseract                    | Accepted | 2026-08-01 | Apple Vision for macOS v1.                                                                                                                 |
| ADR-003 | Storage: SQLite vs JSON                           | Accepted | 2026-08-01 | SQLite (pure-Go `modernc.org/sqlite`, no cgo).                                                                                             |
| ADR-004 | Cloud LLM (OpenAI-compatible) + keys              | Accepted | 2026-08-01 | Universal adapter, apiKey in AES-256-GCM (master key in Keychain), consent + indicator in the overlay.                                     |
| ADR-005 | Local inference without cgo (except whisper.cpp)  | Proposed | 2026-08-01 | STT — whisper.cpp via the official Go binding (the only cgo place). LLM — llama.go (pure Go, vision). LLM port extended to images.         |
| ADR-006 | Overlay and Hotkeys ports                         | Proposed | 2026-08-01 | Window and global shortcuts modeled as ports (stage-2 TDD). Toggleable click-through ⇄ interactive by shortcut.                            |
| ADR-007 | Audio pipeline                                    | Proposed | 2026-08-01 | Sources: microphone + system sound (ScreenCaptureKit). Endpoint detection (whisper.cpp). Cancel on new input.                              |
| ADR-008 | macOS permissions                                 | Proposed | 2026-08-01 | Microphone, screen recording, Accessibility. Centralized module + `app:permission` event.                                                  |
| ADR-009 | Narrow storage interfaces (ISP)                   | Accepted | 2026-08-01 | `port.Storage` split into `SessionStorage`, `AgentStorage`, `SettingsStorage`. Usecases depend only on what they need, test mocks minimal. |
| ADR-011 | Local LLM via Ollama (separate process)           | Accepted | 2026-08-03 | Ollama app as local LLM (REST on `localhost:11434`), streaming `LLM` port; supersedes the LLM part of ADR-005 (llama.go).                  |
| ADR-012 | System sound: SCContentFilter scope + dev signing | Accepted | 2026-08-07 | SCK filter includes all apps (reliable buffers). Dev builds need a stable signing identity (SCError 1003 on Tahoe for unsigned binaries).  |

## How to add an ADR

1. Copy the template: `adr/NNN-short-title-RFC.md` (replace `NNN` with the number, `short-title` with the file name).
2. Fill in all sections.
3. Set the status to **Proposed**.
4. Add a row to the table above.
5. After review and approval the status → **Accepted** (or **Rejected** / **Superseded**).

## Statuses

| Status     | When                                            |
| ---------- | ----------------------------------------------- |
| Proposed   | ADR written, not reviewed yet.                  |
| Accepted   | Owner approved. Implementation relies on it.    |
| Rejected   | Alternative rejected. Arguments in the section. |
| Superseded | Archive. Link to the superseding ADR.           |

## Template

```markdown
# ADR-NNN: [Title]

**Date:** DD-MM-YYYY
**Status:** Proposed
**Related documents:** (list of files it affects)

---

## Context

Briefly: what the problem is, what constraints exist. Link to NFR / spec.

## Options

### A. [Option]

Pros: ...
Cons: ...

### B. [Option]

...

## Selection criteria

| Criterion | A   | B   |
| --------- | --- | --- |
|           |     |     |

## Decision

Option **X** chosen.

## Trade-offs

What we lose, what traces remain in code / architecture.
```
