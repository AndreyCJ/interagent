# ADR-005: Local inference — without cgo (exception: whisper.cpp)

**Date:** 2026-08-01
**Status:** Proposed
**Related documents:** 01-tz.md §6–§8, 02-nfr.md (NFR-01, NFR-08), 05-architecture.md §2–§3, ADR-001, ADR-003, ADR-004

---

## Context

ADR-001 chose: STT — strictly local (whisper.cpp), LLM — hybrid (local llama.cpp or cloud OpenAI-compatible). ADR-003 chose SQLite without cgo (`modernc.org/sqlite`) to keep CI builds simple.

Owner requirement (2026-08-01): **no cgo bindings except whisper.cpp**. LLM inference and the rest of the core must use Go tooling. At the same time, multimodal must be supported (screenshot as-is → LLM, see 01-tz stage 4) — the local LLM must accept images.

## Options

### A. STT: official whisper.cpp Go binding (cgo) [chosen]

`bindings/go` from `ggml-org/whisper.cpp`. This is a cgo wrapper over C++.

**Pros:**

- One binary, no external process for STT.
- Streaming mode + endpoint detection (phrase end detection) out of the box — needed for NFR-01 and ADR-007.
- Official repository, actively maintained.

**Cons:**

- cgo: complicates cross-compilation, but works fine on `macos-latest` CI (clang is available by default).
- The single exception to the "no cgo" rule — requires an explicit freeze.

### B. STT: whisper.cpp as an external process

`whisper-server` runs as a child process (`--server --stream`), communication over localhost.

**Pros:**

- No cgo in our binary.

**Cons:**

- A separate binary must be distributed/downloaded (size, versions, signatures).
- Harder lifecycle management, start/stop, crash handling.
- An additional point of failure (port, pipe).

### C. STT: pure-Go / WASM ports (go-whisper, wazero)

**Pros:**

- Fully pure Go, no external dependencies.

**Cons:**

- Unproven performance; risk of missing NFR-01 (target 2 s, limit 5 s) on M1 8 GB.
- Less maintenance, worse language coverage and endpoint detection.

### D. LLM: llama.go (pure Go) [chosen]

`github.com/ollama/llama.go` — a clean Go port of llama.cpp (GGUF, Metal, KV-cache). Includes vision (LLaVA) for multimodal.

**Pros:**

- Pure Go, consistent with the "no cgo" requirement.
- Vision support — covers multimodal for the local LLM.
- Metal acceleration on Apple Silicon.

**Cons:**

- May lag behind llama.cpp in support for the latest models (risk — monitor).
- Speed on complex models may be lower than native llama.cpp.

### E. LLM: llama.cpp via cgo

**Cons:**

- A second cgo place in the project, violates the owner's requirement. Rejected.

## Selection criteria

| Criterion              | STT A (cgo)    | STT B (process) | STT C (pure/WASM) | LLM D (llama.go) | LLM E (cgo llama.cpp) |
| ---------------------- | -------------- | --------------- | ----------------- | ---------------- | --------------------- |
| "No cgo" requirement   | ❌ (exception) | ✅              | ✅                | ✅               | ❌                    |
| Single binary          | ✅             | ❌              | ✅                | ✅               | ✅                    |
| NFR-01 (latency)       | ✅             | ⚠️ (IPC)        | ⚠️ (risk)         | ⚠️               | ✅                    |
| Multimodal             | —              | —               | —                 | ✅ (vision)      | ✅                    |
| Operational simplicity | ✅             | ⚠️              | ✅                | ✅               | ✅                    |

## Decision

- **STT — whisper.cpp via the official Go binding** (`bindings/go`, cgo). This is the **only** cgo place in the project, frozen as an exception.
- **Local LLM — llama.go** (pure Go, GGUF, Metal). Acceptance criterion: vision support (LLaVA/llava models) for the multimodal scenario. If by stage 4 vision in llama.go turns out to be non-functional — return to this ADR and reconsider.
- **Cloud LLM — OpenAI-compatible** (unchanged, ADR-004).
- **Project rule:** _"Pure Go where possible; cgo allowed only for whisper.cpp"_. ADR-003 is reformulated accordingly.

### `LLM` port extension (multimodal)

The `LLM.Complete(prompt, history)` port is extended to accept an image:

```
type LLMInput struct {
    Text   string
    Image  []byte   // optional: PNG/JPEG of the screenshot
    Format string   // "png" | "jpeg" (if Image != nil)
}

interface LLM {
    Complete(input LLMInput, history []port.Message) (string, error)
    Cancel() error
}
```

The port change is implemented through this ADR. The frontend contract is updated in `06-bind-contracts.md`.

## Trade-offs

- cgo for whisper.cpp is a deliberate exception; builds and tests on `macos-latest` (CI) confirm it.
- llama.go may not support a specific fresh model — for v1, freeze a supported model list (3–8B, GGUF) and show it in the UI.
- Multimodal with a cloud agent means sending the screenshot to the cloud — see the NFR-02 and ADR-004 updates.

---

**Amendment (2026-09-07, Linux audio; amended 2026-09-10):** Linux audio capture adds a second frozen
cgo exception in `internal/adapter/audio`: `capture_pulse_linux.go` (libpulse-simple reader used for
both the mic default source and the system default-sink monitor). Same freeze rules as whisper.cpp:
thin native surface only, all runtime logic pure Go, guarded by `//go:build linux && cgo`. The former
PipeWire fd bridge and the pure-Go XDG portal D-Bus flow were removed with the portal consent model
(ADR-013 amendment). Linux build deps: `libpulse-dev`. This supersedes the "only cgo place" / "cgo
allowed only for whisper.cpp" wording above — whisper.cpp remains the only STT cgo place. Full
record: ADR-013.

**Build note (2026-09-09):** the whisper binding now builds against the upstream submodule in-place
(`third_party/whisper.cpp/bindings/go` — the previous fork `third_party/whisper-bindings` was
deleted). The binding's cgo directives reference the whisper libs by name only, so builds need the
CGO env exported — `source scripts/whisper-env.sh` (`CGO_CPPFLAGS`/`CGO_LDFLAGS`/`LIBRARY_PATH`
pointing at `third_party/whisper.cpp/{dist/include,dist/lib)`). Rebuilding the libs:
`./scripts/build-whisper.sh`. `.github/workflows/test.yml` sources the script for the `go`,
`go-linux` and `e2e` jobs.

**Amendment (2026-09-15, STT GPU):** whisper.cpp remains the only STT cgo place, but it may now
offload to the GPU: `scripts/build-whisper.sh` enables `-DGGML_VULKAN=ON` on Linux when the
Vulkan toolchain is present (CPU fallback otherwise), and `scripts/whisper-env.sh` exports
`-lggml-vulkan -lvulkan`. The binding is used unpatched; `libggml-vulkan.a` is merged into the
aggregate `libggml.a` at build time so the directive-only link list resolves. Model sizes,
defaults and voxtype model reuse: ADR-014.
