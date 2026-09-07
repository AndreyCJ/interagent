# Interagent

An overlay app for interviews and presentations: it listens to system and microphone
audio (STT), captures screenshots (OCR), sends the text to a cloud OpenAI-compatible
LLM, and shows cues in a transparent window on top of all windows.

## Getting started

Requirements: Go 1.25+, Node 22 + pnpm.

- **macOS 14+**: the audio dev loop requires a signed bundle — see AGENTS.md (the macOS section).
- **Linux/Arch**: install `webkit2gtk-4.1`, `gtk3`, `libayatana-appindicator`.
  Wails v2.15 builds against webkit2gtk-4.1 (the 4.0 ABI is not in Arch's official
  repos), so every wails command must be run with the `webkit2_41` build tag.
  Audio capture additionally needs `libpulse-dev` and `libpipewire-0.3-dev`
  (pkg-config files for the cgo pulse-simple + PipeWire portal bridge).

```
pnpm install                 # pnpm workspace (repo root): frontend + git hooks
pnpm --dir frontend build    # go:embed needs frontend/src/app/dist (before Go checks)
wails dev -tags "webkit2_41"    # dev mode
wails build -tags "webkit2_41"  # production build (macOS → build/bin/*.app, Linux → build/bin/interagent)
```

Cloud LLM is configured via the `LLM_API_KEY`/`LLM_BASE_URL`/`LLM_MODEL` env vars
(env override in `app.go`). macOS audio loop: `LLM_API_KEY=... ./scripts/dev-audio.sh`
(builds and signs the `.app` with a stable identity — `wails dev` does not work for
audio on macOS).

**Linux notes.** `wails doctor` reports "Required dependencies missing: libwebkit" —
it only probes the 4.0 ABI; this is safe to ignore. On a Wayland session launch with
`GDK_BACKEND=x11` (the native GTK Wayland backend emits "Protocol error dispatching
to Wayland display").

## Checks

```
go test ./... && go vet ./... && go fmt ./...     # Go
cd frontend && pnpm lint && pnpm format:check && pnpm test
cd frontend && pnpm exec playwright test           # e2e
```
