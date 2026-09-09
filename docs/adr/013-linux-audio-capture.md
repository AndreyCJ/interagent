# ADR-013: Linux system audio capture (pulse-simple + XDG portal ScreenCast) and permission mapping

**Date:** 07-09-2026
**Status:** Proposed
**Related documents:** ADR-005 (second frozen cgo exception), ADR-007/008 (Linux notes),
`docs/plans/step-1-linux-audio.md`

---

## Context

macOS capture (ScreenCaptureKit / AVFoundation) and TCC permissions have no Linux analogue. The app
must capture the microphone and system sound on Linux and report permissions honestly — never a fake
grant.

## Options

### A. Pure-Go via third-party pulse/pipewire bindings

Pros: no cgo.
Cons: partial/unmaintained bindings; portal fd transport and stream negotiation are awkward.

### B. cgo for pulse-simple + minimal PipeWire fd bridge (portal flow pure Go) [chosen]

Pros: stable, a small native surface; mirrors the darwin ObjC-bridge pattern (thin native layer,
runtime logic rationalized in pure Go).
Cons: a second frozen cgo exception (amends ADR-005); Linux build needs `libpulse-dev` /
`libpipewire-0.3-dev`.

### C. PipeWire only, custom consent flow

Cons: a bigger cgo surface; mic sources are still pulse-shaped; no portal consent story.

## Selection criteria

| Criterion               | A   | B     | C      |
| ----------------------- | --- | ----- | ------ |
| Native surface size     | 0   | small | large  |
| Baseline stability      | low | high  | medium |
| Honest permission model | n/a | yes   | no     |
| Fits roadmap            | low | yes   | medium |

## Decision

Option **B**. Mic = pulse default source via `libpulse-simple`; system sound = the XDG Desktop
Portal ScreenCast audio stream (the portal picker is the consent), streamed over the session's
PipeWire fd via a minimal cgo node bridge. **Deliberate: no default-sink (`.monitor`) fallback** —
if the ScreenCast portal is missing, the app errors honestly instead of silently capturing the wrong
source.

### Build layout (as shipped)

- `internal/adapter/portal` — pure-Go godbus ScreenCast flow (CreateSession → SelectSources with
  `persist_mode=2` → Start → OpenPipeWireRemote), yielding a `Stream` (PipeWire fd + node id +
  restore token). No build tag; the fd close is platform-split in `sys_unix.go`/`sys_windows.go`.
- `internal/adapter/audio/capture_pulse_linux.go` — libpulse-simple mic reader + source enumeration,
  `//go:build linux && cgo`.
- `internal/adapter/audio/capture_pipewire_linux.go` — minimal PipeWire node bridge
  (`pw_context_connect_fd` on the portal fd, F32 48 kHz, down-mix to mono), `//go:build linux && cgo`.
- Honest `linux && !cgo` fallbacks surface "built without cgo" errors; the pure-Go Linux logic
  (device mapping, pulse-reachability probe) lives in `capture_linux.go` (`//go:build linux`).

The permissions adapter consumes the portal behind an interface seam (`screenCast`:
`Available`/`Grant`/`Granted`) plus a `func() error` mic probe — both wired in `app.go`, so
`adapter/audio` and `adapter/system` never import each other.

## Permission mapping (never lies)

- `microphone` → transport probe: a pulse daemon socket is reachable (`PulseReachable()`, pure-Go
  stat on `$XDG_RUNTIME_DIR/pulse/native` / `/tmp/pulse/native`) — no consent gate for native apps.
  The Camera portal flow (`org.freedesktop.portal.Camera`, for sandboxed apps) is intentionally
  deferred to Step 2.
- `screen-recording` → a ScreenCast session was completed this run, or a persisted restore token
  exists (`portal.Granted()`). `Request`/`OpenSettings` run the picker flow (`portal.Grant()`); the
  picker IS the grant dialog.
- `accessibility` → always `false` (no Linux equivalent); `Request`/`OpenSettings` error honestly.

## Restore-token persistence

`$XDG_DATA_HOME/interagent/screencast-restore-token` (falls back to
`~/.local/share/interagent/screencast-restore-token` when `XDG_DATA_HOME` is unset). Best-effort:
backends that do not supply a token simply show the picker again on the next session start.

## Trade-offs

- Second frozen cgo exception; whisper.cpp remains the only STT cgo place (ADR-005 amendment).
- Native Linux apps skip the sandbox consent model — the honest substitute is a transport probe, not
  a consent claim.
- A missing portal or pulse daemon yields actionable errors, never fake grants.
- `internal/adapter/portal` is shared by the audio and permissions adapters with no
  adapter↔adapter imports.
