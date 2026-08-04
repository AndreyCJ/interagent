# Documentation Map

## Statuses

| Status     | Meaning                                                         |
| ---------- | --------------------------------------------------------------- |
| Draft      | Under review, may change. Do not rely on it for implementation. |
| Approved   | Agreed with the owner. Use as source of truth.                  |
| Deprecated | No longer relevant. Replaced by another document.               |

## Documents

| File                   | Purpose                                                                              | Status   | Audience       |
| ---------------------- | ------------------------------------------------------------------------------------ | -------- | -------------- |
| `01-tz.md`             | Spec: product goal, entities, scenarios, glossary, integrations, stages              | Approved | Human + Agent  |
| `02-nfr.md`            | Non-functional requirements (latency, CPU, RAM, click-through, graceful degradation) | Approved | Human + Agent  |
| `03-process.md`        | Development process: TDD, review, Definition of Done, CI                             | Approved | Human + Agent  |
| `04-events.md`         | Runtime events backend → frontend (types and bind methods — in code)                 | Approved | Agent (+Human) |
| `05-architecture.md`   | Architecture: layers, dependencies, pipeline, window, errors, storage                | Approved | Human + Agent  |
| `06-bind-contracts.md` | Contract of bind methods frontend ↔ backend (input, output, errors, events)          | Approved | Agent (+Human) |
| `adr/001-...md`        | ADR-001: STT local (whisper.cpp); LLM — local or cloud, user's choice                | Accepted | Human + Agent  |
| `adr/002-...md`        | ADR-002: OCR — Apple Vision (macOS v1)                                               | Accepted | Human + Agent  |
| `adr/003-...md`        | ADR-003: storage — SQLite (pure Go, no cgo)                                          | Accepted | Human + Agent  |
| `adr/004-...md`        | ADR-004: cloud LLM (OpenAI-compatible), apiKey encryption, consent                   | Accepted | Human + Agent  |
| `adr/005-...md`        | ADR-005: inference without cgo (except whisper.cpp), llama.go, multimodal port       | Proposed | Human + Agent  |
| `adr/006-...md`        | ADR-006: Overlay and Hotkeys ports, toggleable click-through mode                    | Accepted | Human + Agent  |
| `adr/007-...md`        | ADR-007: audio pipeline (SCK, endpoint detection, cancel)                            | Accepted | Human + Agent  |
| `adr/008-...md`        | ADR-008: macOS permissions (microphone, screen recording, Accessibility)             | Accepted | Human + Agent  |
| `adr/009-...md`        | ADR-009: narrow storage interfaces (ISP)                                             | Accepted | Human + Agent  |
| `adr/010-...md`        | ADR-010: events port (Event Bus) for async usecase → frontend events                 | Accepted | Human + Agent  |
| `adr/011-...md`        | ADR-011: local LLM via Ollama (separate process)                                     | Accepted | Human + Agent  |

## What is the source of truth (rule for agents)

1. **Runtime events** (backend → frontend) — described in `04-events.md`.
2. **Principles, layers, dependencies** — only in `05-architecture.md`.
3. **Bind methods and their contract** — only in `06-bind-contracts.md` (data types — in code).
4. **Decisions about integrations / inference / storage** — only in ADRs. If mentioned in 01-tz — there is a link there (ADR-XXX).
5. **STT — local only (whisper.cpp).** LLM — local (Ollama, ADR-011) or cloud OpenAI-compatible, user's choice (see ADR-001, ADR-004, ADR-011). apiKey is stored encrypted.
6. **Changing the architecture** — only via ADR (`docs/adr/README.md`).

## Reading order

1. `01-tz.md` — understand why the project exists.
2. `02-nfr.md` — understand the behavioral constraints.
3. `docs/adr/001-local-vs-api-llm.md` — key decision: STT local, LLM hybrid.
4. `docs/adr/004-cloud-llm.md` — cloud LLMs, apiKey encryption, consent.
5. `docs/adr/005-inference-without-cgo.md` — inference integrations without cgo, multimodal.
6. `05-architecture.md` — how the layered stack and pipeline work (types — in code, events — `04-events.md`).
7. `06-bind-contracts.md` — bind methods contract.
8. `04-events.md` — runtime events contract.
9. `03-process.md` — how to work (TDD, review, DoD).
