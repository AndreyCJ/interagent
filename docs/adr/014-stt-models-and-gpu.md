# ADR-014: STT model sourcing and GPU backend

**Date:** 2026-09-15
**Status:** Proposed
**Related documents:** ADR-001, ADR-005, ADR-007, `internal/adapter/models`, `scripts/build-whisper.sh`

---

## Context

Base-level whisper models (`ggml-base`) produce empty/`[BLANK_AUDIO]` results on real
interview audio (system-sound capture through the default-sink monitor). Fixing the
transcription quality needs (1) a larger default model and (2) GPU inference so a
~3 GB model keeps up with real-time streaming. The user already owns the exact
`ggml-large-v3.bin` whisper.cpp model through voxtype and should not download it again.

Constraints: whisper.cpp stays the only STT cgo place (ADR-005); the Go binding is
used in-place as a git submodule and must not be patched; CI (no Vulkan toolchain,
no GPU) must keep working unchanged.

## Options

### A. Reuse voxtype's installed model + downloadable fallback [chosen]

Before any download, `models.Store.Download` looks in the voxtype models directory
(`$XDG_DATA_HOME|~/.local/share/voxtype/models`, override `INTERAGENT_VOXTYPE_MODELS_DIR`);
if the exact file exists **and** its SHA-256 matches `checksums.txt`, it is linked
(hardlink, copy on cross-device) into the app's models dir and the download is skipped.
Checksum mismatch or absence → normal download.

Pros: zero extra bandwidth for the common case; verified integrity (same hash gate as downloads); self-heals if voxtype is later removed (hardlink keeps the inode).
Cons: reads another app's cache directory (read-only), adds a hardlink/copy code path.

### B. Always download into the app models dir

Pros: simplest.
Cons: 3 GB duplicate download for users who already have the model via voxtype.

### C. Vulkan GPU backend, auto-detected at build time [chosen]

`scripts/build-whisper.sh` enables `-DGGML_VULKAN=ON` on Linux only when the Vulkan
toolchain is present (`vulkan-headers`+`spirv-headers` dev packages plus a shader
compiler). The Go binding needs no changes: `Whisper_init` uses
`whisper_context_default_params()` (`use_gpu=true`), and `whisper_backend_init_gpu`
auto-selects the GPU device with a silent CPU fallback. Because the binding's cgo
directives never link `-lggml-vulkan`, the build `ar -M`-merges `libggml-vulkan.a`
into the aggregate `libggml.a` (the env `CGO_LDFLAGS` set by `whisper-env.sh` are
placed before the directive archives by `go`, which would otherwise leave
`ggml_backend_vk_reg` unresolved in multi-cgo links). CPU-only builds are unchanged.

Pros: large models fit within real-time budget on the RTX 3070; macOS/CI untouched.
Cons: GPU VRAM shared with the desktop; on memory pressure ggml spills to host memory (degrades, does not crash).

## Decision

- Default STT model becomes **`large-v3`** (existing DBs seeded with `base` are migrated on open). `large-v3-turbo` is offered as a lighter option; downloader specs + checksums added; settings validation, `sttModelKey`, storage seed, and the settings UI updated together.
- Model install adopts the **voxtype-reuse-first** flow (A) with the checksum gate.
- STT compute uses the **Vulkan GPU backend** when the toolchain is present (C); otherwise CPU. Documented in README.

## Trade-offs

- Any existing user who deliberately chose `base` gets upgraded to `large-v3` once per DB (the setting is one dropdown click to change back; `base` remains installable).
- The 2.9 GB `ggml-large-v3.bin` is hashed once at adoption (~3 s) before linking.
- Reading voxtype's directory is read-only; voxtype remains the probed app for its own data.
