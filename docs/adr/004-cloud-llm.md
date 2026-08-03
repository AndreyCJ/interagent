# ADR-004: Cloud LLM (OpenAI-compatible providers) and key storage

**Date:** 2026-08-01
**Status:** Accepted
**Related documents:** 01-tz.md §2/§6/§8, 02-nfr.md (NFR-02, NFR-11), 05-architecture.md §3, ADR-005 (LLM port extension for images). AgentConfig types — in code: `internal/port/types.go`.

---

## Context

STT stays local (whisper.cpp — see ADR-001). But the quality of local LLMs (3–8B) is inferior to cloud ones, especially on complex tasks. The user wants a choice: sometimes prefer the speed and quality of a cloud LLM, sometimes the privacy of a local one.

Requirements:

- Cloud LLM — only OpenAI-compatible endpoints (OpenAI, Groq, LM Studio, Ollama remote, vLLM, etc.). Native providers (Anthropic) — not v1.
- `apiKey` must be stored locally and **encrypted**.
- Sending text to the cloud — **only when explicitly enabled** by the user, with visual indication.
- Audio and screenshots never go to the network (see NFR-02).

## Options

### A. Universal OpenAI-compatible client (Recommended)

One HTTP adapter `adapter/llm/openai.go`, parameters: `baseUrl`, `model`, `apiKey`. Any OpenAI-compatible service — via the `baseUrl` setting.

**Pros:**

- One code path for all compatible providers (OpenAI, Groq, LM Studio, Ollama, vLLM, DeepSeek, etc.).
- Easily extensible: a new model is `model` + `baseUrl`, no new code.
- Minimal code, minimal surface area for bugs.

**Cons:**

- Cannot adapt to provider-specific features (formatters, tool-calling specifics).

### B. A separate adapter per provider

`openai.go`, `anthropic.go`, `gemini.go` — each with its own request structure.

**Pros:**

- Access to provider-specific capabilities.

**Cons:**

- Harder to maintain and test.
- v1 does not need Anthropic/Gemini (the user asked for free models and OpenAI-compatible ones).

### C. Local LLM only

Give up the cloud entirely.

**Cons:**

- Limits the scenario: local LLM quality is worse; the user is forced to download models.

## Selection criteria

| Criterion            | A (OpenAI-compatible)    | B (per-provider) | C (local) |
| -------------------- | ------------------------ | ---------------- | --------- |
| v1 coverage          | ✅ (all free-compatible) | ✅               | ❌        |
| Implementation speed | ✅                       | ❌               | ✅        |
| Anthropic support    | ❌ (added later)         | ✅               | ❌        |
| Code simplicity      | ✅                       | ❌               | ✅        |

## Decision

Option **A — universal OpenAI-compatible adapter** chosen.

### How to store apiKey

- In SQLite (see ADR-003) **only encrypted** (AES-256-GCM).
- The encryption master key is stored in **macOS Keychain** (service `com.interagent.keys`, account `master`).
- On first launch the master key is generated randomly and saved to Keychain.
- `apiKey` in `AppSettings` / `AgentConfig` is always `ciphertext`, **never plaintext**.

### Consent model (NFR-02)

- The provider is chosen **at the agent level** (`AgentConfig.provider = "openai-compatible"` + `apiKey` + `baseUrl`).
- On the **first** enabling of a cloud provider — a modal warning: "Transcription and OCR text, as well as screenshots (if the mode of sending images to the model is enabled), will be sent to the endpoint you choose. Audio never leaves the device." Confirmation — `OK`.
- While a cloud provider is active — an indicator is **always** shown in the overlay (e.g., "☁️" next to the model). If direct-image screenshot mode is active (multimodal, ADR-005) — the indicator also shows that images go to the cloud.
- Cloud mode can only be disabled through the agent settings.

### What goes to the network

| Data                       | Leaves?                                                                      | When                               |
| -------------------------- | ---------------------------------------------------------------------------- | ---------------------------------- |
| Audio (PCM/mic/system)     | ❌ Never                                                                     | —                                  |
| Screenshot (image)         | ✅ Only if `provider = openai-compatible` **and** direct-image mode selected | → `baseUrl` of the chosen provider |
| Text (transcription / OCR) | ✅ Only if `provider = openai-compatible`                                    | → `baseUrl` of the chosen provider |

## Trade-offs

- Native cloud providers (Anthropic, Google) — not v1. We will add them later if needed.
- With a cloud provider, latency/CPU/RAM depend on the network and a third-party service — see NFR-01/03/07 (updated).
- Storing the master key in Keychain ties encryption to a single macOS account.
