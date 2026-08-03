# ADR-001: Local LLM/STT inference vs Cloud APIs

**Date:** 2026-07-29
**Status:** ✅ Accepted
**Changed:** 2026-08-01 (LLM decision extended with hybrid mode)
**Related documents:** 01-tz.md §6–§7, 02-nfr.md (NFR-02, NFR-11), 05-architecture.md §3

---

## Context

The app needs two key external services:

- **STT** (Speech-to-Text) — converting the interviewer's voice to text
- **LLM** (Large Language Model) — generating a response from the transcription or screenshot

Both services can run locally (whisper.cpp, llama.cpp) or use cloud APIs (OpenAI, Anthropic). Each approach has trade-offs affecting the architecture, UX, and app distribution.

## Options

### A. Fully local inference

| Component | Technology                                                |
| --------- | --------------------------------------------------------- |
| STT       | whisper.cpp (C++ bindings via cgo or an external process) |
| LLM       | llama.cpp / llama.go / Ollama (REST on localhost)         |

**Pros:**

- Full confidentiality — data never leaves the device
- Works offline
- No API costs (one-time hardware cost)
- No account / API key needed

**Cons:**

- Significant RAM/VRAM usage (at least 4–8 GB for a decent model)
- Notable CPU/GPU load (may interfere with work in an IDE)
- Response speed lower than APIs (especially on Apple Silicon without Metal)
- App binary grows (C/C++ bindings needed)
- Complex cross-compilation and build
- The user must download model files (gigabytes)

### B. Cloud APIs

| Component | Technology                                                |
| --------- | --------------------------------------------------------- |
| STT       | OpenAI Whisper API / Deepgram                             |
| LLM       | OpenAI Chat Completion / Anthropic Claude / Google Gemini |

**Pros:**

- Minimal load on the device
- High speed and quality of responses (best models)
- Simple integration — HTTP request with a key
- Small binary, simple build
- Models are always up to date, no manual updates

**Cons:**

- Internet required
- Data goes to the provider's server (confidentiality in question)
- Cost per token (PAYG)
- Account and API key with a payment method required
- Vendor lock-in

### C. Local STT + hybrid LLM (user's choice) [chosen]

Setting: `AgentConfig.provider = 'local' | 'openai-compatible'`. STT is always local (whisper.cpp).

**Pros:**

- The user chooses the privacy/speed balance for the LLM themselves.
- STT stays fully local — audio never goes to the network.
- Can start with a local LLM and switch to cloud when needed.

**Cons:**

- Two LLM backends must be maintained (local + HTTP client).
- More complex code: abstractions, adapters, error handling for both paths.
- With a cloud LLM, text data leaves the device (with the user's consent and an indicator).

## Selection criteria

| Criterion                | A (local)    | B (cloud for all) | C (local STT + hybrid LLM)      |
| ------------------------ | ------------ | ----------------- | ------------------------------- |
| Audio/screenshot privacy | ✅           | ❌                | ✅ (STT local, LLM — choice)    |
| Offline mode             | ✅           | ❌                | ⚠️ (local LLM offline)          |
| Response speed           | ⚠️ (3-10 s)  | ✅ (1-3 s)        | ⚠️/✅ (depends on LLM provider) |
| Device load              | ❌ (high)    | ✅ (low)          | ⚠️ (depends)                    |
| Development complexity   | ❌ (high)    | ✅ (low)          | ⚠️ (medium)                     |
| Cost for the user        | ✅ (free)    | ❌ (PAYG)         | ⚠️ (optional)                   |
| Binary size              | ❌ (200+ MB) | ✅ (small)        | ❌ (local models)               |

## Decision

- **STT — strictly local.** `whisper.cpp` (C++ bindings or an external process). Cloud STT is not supported. _The concrete integration is chosen in ADR-005 (official Go binding, cgo)._
- **LLM — hybrid (user's choice).**
  - **Local:** `llama.cpp / llama.go` (CPU/Metal inference). _`llama.go` (pure Go) chosen — see ADR-005._
  - **Cloud:** OpenAI-compatible providers of the user's choice (OpenAI, Groq, LM Studio, Ollama, etc. via `baseUrl` + `apiKey`). See ADR-004.

### Rationale

1. **Privacy — critical for audio/screenshots.** The interview scenario requires microphone data and screenshots to never leave the device. Therefore **STT is always local** (whisper.cpp): audio never goes to the network.
2. **LLM — the user decides.** Text (transcription or OCR) may be sent to a cloud LLM **only if the user explicitly enabled a cloud provider for the agent** (see ADR-004). This is accompanied by an indicator in the overlay.
3. **Offline mode.** Interviews happen in offices with corporate VPNs too. A local LLM works without the network.
4. **Cost.** Local inference is free after the one-time hardware purchase. Cloud is PAYG and adds up quickly. The user chooses when to pay.
5. **Apple Silicon.** M-series provides enough performance for whisper.cpp + small LLMs (3–8B) via Metal.

### Trade-offs

- Higher barrier to entry: the local model must be downloaded (several GB).
- The local LLM is inferior to GPT-4/Claude 3.5 in quality — acceptable for hints, but noticeable for complex tasks.
- On older Intel Macs local LLM response time may exceed 5 s — show a warning and recommend the M-series (or switching to a cloud provider).
- With a cloud LLM, text data leaves the device (per the user's explicit choice and with an indicator). Microphone and screenshot data never leave the device.

### Links to other ADRs

- **ADR-002** chooses the OCR engine (Apple Vision) for macOS v1.
- **ADR-003** chooses SQLite (pure Go, no cgo) for local storage.
- **ADR-004** chooses the cloud LLM abstraction (OpenAI-compatible), apiKey encryption, and the consent model.
