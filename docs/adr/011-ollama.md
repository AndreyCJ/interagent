# ADR-011: Local LLM via Ollama (separate process)

**Date:** 2026-08-03
**Status:** Accepted
**Supersedes:** ADR-005 (LLM part: llama.go in-process). STT decision of ADR-005 is unchanged (whisper.cpp cgo).
**Related documents:** 01-tz.md §6–§8, 05-architecture.md §3, ADR-001, ADR-004, ADR-007

## Context

ADR-005 chose llama.go (pure Go) for local LLM inference. In practice the user
runs models via the Ollama app, which also manages model downloads. Bundling an
in-process inference engine duplicates Ollama and forces our app to download
several-GB GGUF models. The user wants the app to just talk to Ollama.

## Decision

- Local LLM = **Ollama** running as a separate process/app, native REST on
  `http://localhost:11434`:
  - `/api/chat` (streaming) — generate answers;
  - `/api/tags` — list pulled models (availability check);
  - `/api/cancel` — cancel in-flight generation.
- Our app does **not** download GGUF models; the Ollama app manages models
  (`ollama pull`).
- The `LLM` port is extended to streaming and carries `LLMInput.Language`
  (whisper-detected) so answers match the interviewer's language.
- Multimodal (stage 4): Ollama vision models (llava / qwen2.5-vl) satisfy the
  direct-image requirement.

## Trade-offs

- Requires the Ollama app to be installed/running; a missing Ollama is a clear
  `app:error` with a retry (NFR-06).
- In-process llama.go gives up; single binary cost is accepted in exchange for
  not distributing/embedding an inference engine.
