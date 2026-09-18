# Interagent

An overlay app for interviews and presentations: it listens to system and microphone
audio (STT), captures screenshots (OCR), sends the text to a cloud OpenAI-compatible
LLM, and shows cues in a transparent window on top of all windows.

## Getting started

Requirements: Go 1.25+, Node 22 + pnpm.

First run: `pnpm install` and `pnpm --dir frontend build` (go:embed needs
`frontend/src/app/dist`).

The `scripts/dev.sh`/`scripts/build.sh` wrappers source `scripts/whisper-env.sh`
(CGO include/lib paths for the whisper.cpp binding) and build the native libs on
first run. Calling `wails`/`go` directly requires `source scripts/whisper-env.sh`
first, or any Go build fails with `whisper.h: No such file or directory`.

### macOS 14+

No build tags — Wails uses the system WebKit (WKWebView).

Dev: hot-reload window (UI only, unsigned):

```
./scripts/dev.sh
```

Audio dev loop (signed `.app`, capture works):

```
./scripts/dev-macos.sh
```

Prod build → `build/bin/interagent.app`:

```
./scripts/build.sh
```

Run the prod bundle:

```
open build/bin/interagent.app
```

`wails dev` runs an **unsigned** bare binary — ScreenCaptureKit and TCC refuse
it, so system/mic audio won't work there; the audio loop is `dev-macos.sh`
(signed identity, so the recording grants persist across rebuilds, ADR-012).
Release distribution (Developer ID + notarization) is a separate pipeline.

### Linux (webkit2gtk-4.1)

Install `webkit2gtk-4.1`, `gtk3`, `libayatana-appindicator`; for audio capture
also `libpulse-dev`. STT offloads to an NVIDIA/AMD GPU (Vulkan) when the dev
packages are present, so install them for the app to transcribe on the GPU:

```
sudo pacman -S vulkan-headers spirv-headers shaderc glslang
```

`scripts/build-whisper.sh` detects them and enables `GGML_VULKAN=ON`;
without them it silently builds a CPU-only whisper (works, just slower).
Wails v2.15 builds against
webkit2gtk-4.1 (the 4.0 ABI is not in Arch's official repos), so **every**
command needs the `webkit2_41` build tag.

Dev: hot-reload window:

```
./scripts/dev.sh -tags "webkit2_41"
```

Prod build → `build/bin/interagent`:

```
./scripts/build.sh -tags "webkit2_41"
```

Run the prod binary:

```
./build/bin/interagent
```

Wayland: launch with `GDK_BACKEND=x11`. `wails doctor` reporting "Required
dependencies missing: libwebkit" only probes the 4.0 ABI — safe to ignore.

### LLM key (both platforms)

Dev: the cloud LLM reads `LLM_API_KEY`/`LLM_BASE_URL`/`LLM_MODEL` from the env
(env override in `app.go`). Prepend them to the platform's dev command from the
sections above, e.g. macOS:

```
LLM_API_KEY=... ./scripts/dev.sh
```

Prod: the key comes from the encrypted agent config (master key in Keychain) —
no env needed.

## Checks

```
source scripts/whisper-env.sh
go test ./... && go vet ./... && go fmt ./...     # Go
cd frontend && pnpm lint && pnpm format:check && pnpm test
cd frontend && pnpm exec playwright test           # e2e
```
