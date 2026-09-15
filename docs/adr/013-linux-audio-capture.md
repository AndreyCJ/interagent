# ADR-013: Linux system audio capture (pulse default-sink monitor) and permission mapping

**Date:** 07-09-2026
**Status:** Accepted (amended 2026-09-10 — system sound no longer uses the portal)
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

Mic = pulse **default source** via `libpulse-simple`.

### 2026-09-10 amendment: system sound = pulse default-sink monitor

System sound is captured from the **default sink monitor** (`@DEFAULT_SINK@.monitor`) through the
same `libpulse-simple` reader as the mic, plus a shared pump. A native (non-sandboxed) Linux app
needs **no OS permission** to capture system audio, so `Listen` starts immediately — the previous
XDG Desktop Portal ScreenCast flow (portal picker as consent, PipeWire fd bridge) has been **removed**:
the portal picker is a screen/window-share dialog with no audio-only variant, so gating system audio
on it was wrong. When a future step adds real screen capture (screenshots/OCR) on Linux, that step
reintroduces a ScreenCast flow and its own consent story (ADR-002/ADR-008 notes).

The `internal/adapter/portal` package and `capture_pipewire_linux.go` were deleted. `libpipewire-0.3-dev`
is no longer a build dependency; `libpulse-dev` remains.

### Build layout (as shipped)

- `internal/adapter/audio/capture_pulse_linux.go` — `libpulse-simple` reader + source enumeration
  (mic and system), `//go:build linux && cgo`.
- `internal/adapter/audio/capture_linux.go` — pure-Go Linux logic: shared `pulsePump`, device
  mapping (monitors for system, all sources for mic), pulse-reachability probe (`PulseReachable`),
  honest `linux && !cgo` errors.
- `app.go` wires `audio.NewSystemCapture()` and `system.NewPermissions(audio.PulseReachable)`.

## Permission mapping (never lies)

Linux has no consent model for native apps; permissions are transport probes:

- `microphone` → transport probe: a pulse daemon socket is reachable (`PulseReachable()`, pure-Go
  stat on `$XDG_RUNTIME_DIR/pulse/native` / `/tmp/pulse/native`).
- `screen-recording` → **no OS gate on Linux**: `Status` is always `true`, `Request` is a no-op.
  The capture layer reports transport errors honestly when `Start` actually runs (no silent fake
  grants). On macOS this permission keeps its TCC meaning (ADR-008); `ensureListeningPermissions`
  passes on Linux because no consent is required.
- `accessibility` → always `false` (no Linux equivalent); `Request`/`OpenSettings` error honestly.
- `OpenSettings` (mic + screen-recording) → opens `pavucontrol` (the audio mixer); `accessibility`
  errors honestly. No permission-settings deep links exist on Linux.

## Trade-offs

- Second frozen cgo exception; whisper.cpp remains the only STT cgo place (ADR-005 amendment).
- System audio is only as reliable as `pipewire-pulse`/`pulseaudio` (already required for the mic):
  a missing daemon yields actionable errors, never fake grants.
- macOS keeps TCC/SCK; Linux and macOS sharing `port.Permissions`/`port.AudioInput` means the
  platform semantics diverge inside the adapters, not in the ports.
- The restore-token file (`$XDG_DATA_HOME/interagent/screencast-restore-token`) is obsolete; stale
  files are harmless and ignored.
