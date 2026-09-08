# Session prompt — Step 1: Linux audio capture + permissions (libpulse-simple + XDG Desktop Portal)

Repo: Interagent (Wails v2, Go, Vue 3). Follow AGENTS.md: TDD — tests first, ADR for any
contract/architecture change. THIS is the first of two planned sessions; it runs on its own branch.
Do NOT modify `internal/adapter/stt/**`, `bind_*.go`, or the whisper build wiring. A later session
(Step 2) does the whisper/transcription work and will rebase on top of your merged branch.

## Decisions (locked)

- Capture: **cgo** — `libpulse-simple` for the mic; **XDG Desktop Portal ScreenCast** audio stream
  for system sound (interviewer's app audio; user picks the source in the portal picker). This is a
  SECOND frozen cgo exception -> amend ADR-005.
- Permissions: **XDG Desktop Portal** grants (real consent). No more lying "granted=true".
- System-sound semantics: the portal ScreenCast stream, NOT the default-sink monitor (no silent
  fallback to `.monitor`).

## Context (verified)

- `internal/adapter/audio/capture_other.go` (`//go:build !darwin`) stubs `MicrophoneCapture` and
  `SystemCapture` with "not supported"; `Devices()/SetDevice()` return `(nil,nil)`.
- `internal/adapter/system/permissions_other.go` (`//go:build !darwin`) returns `true` for every
  Status, errors on Request, no-op OpenSettings. `settingsURL()` in `permissions.go` only knows
  macOS deep links.
- Darwin impls in `capture_darwin.go`/`capture_darwin_objc.go`/`permissions_darwin.go` — DO NOT touch.
- Ports (platform-neutral): `internal/port/audio.go` `AudioInput{Start(onChunk func([]byte))/Stop()
/Devices()/SetDevice(id)}`, `AudioDevice{ID,Name,IsDefault}`; `internal/port/permissions.go`
  `Permissions{Status/Request/OpenSettings(perm)}`, `Permission` =
  "microphone"|"screen-recording"|"accessibility".
- Capture contract: float32 mono 48 kHz little-endian PCM chunks in onChunk (STT resamples 48k->16k).
- `github.com/godbus/dbus/v5` already an indirect dep (promote to direct — run `go mod tidy`).
  No other Linux audio libs present.
- Target env (this machine): Omarchy, PipeWire 1.6.8 + pipewire-pulse, Hyprland Wayland,
  xdg-desktop-portal-hyprland running. NOTE: on pure-PipeWire setups without pipewire-pulse,
  `pa_simple` may fail to connect — detect and surface an actionable error.

## Part 1 — cgo foundation + build tags

1. Create `internal/adapter/audio/capture_linux.go` (`//go:build linux`). Narrow `capture_other.go`
   to `//go:build !darwin && !linux` (Windows keeps the stub).
2. Pulse cgo in one small file `capture_pulse_linux.go`: `#cgo pkg-config: libpulse-simple`
   (fallback `-lpulse-simple -lpulse`). Thin cgo reader; runtime logic pure-Go (mirror the darwin
   ObjC-bridge pattern: minimal native surface).
3. `capture_pipewire_linux.go` (`//go:build linux && cgo`): connects the portal's PipeWire fd and
   streams PCM (Part 3).
4. Document Linux build deps (`libpulse-dev`, `libpipewire-0.3-dev`) in README; add/adjust the
   linux build/test job in `.github/workflows/test.yml`. Do NOT change how whisper static libs are
   produced there — Step 2 will rework that; keep it working as-is today.

## Part 2 — MicrophoneCapture (Linux)

- `pa_simple_new(NULL, "interagent", PA_STREAM_RECORD, device|NULL, "record",
spec{Float32LE, 48000, 1}, NULL, &err)`; read loop -> onChunk in ~100 ms float32-mono chunks;
  `Stop()` -> `pa_simple_free`.
- `Devices()`: enumerate `pa_source_info_list` -> source name = ID, description = Name, default
  source `IsDefault=true`. `SetDevice(id)` stores + reopens with that device.
- Failure mode: if the pulse daemon is unreachable (no pipewire-pulse), return an actionable error.
- Unit-test pure-Go parts (device-name/shape mapping) without a live daemon.

## Part 3 — SystemCapture (Linux): XDG portal ScreenCast audio stream

1. DBus portal flow (pure Go, godbus): `org.freedesktop.portal.ScreenCast` —
   CreateSession -> SelectSources (source_type incl. MONITOR/WINDOW, audio) -> Start ->
   on `Request::Response` signal read `streams` `[node_id, props]`; set `persist_mode=2` for a
   restore token (used by Status). Then `OpenPipeWireRemote(session)` returns a PipeWire fd scoped
   to that session.
2. Consume fd via minimal cgo: `pw_context_connect_fd`, `pw_stream_new` + node ops, negotiate
   `SPA_AUDIO_FORMAT_F32, 48000`, dequeue buffers in on_process, down-mix to mono, push onChunk.
   Keep it thin and isolated (same shape as the darwin ObjC bridge).
3. If `org.freedesktop.portal.ScreenCast` isn't available: honest actionable error, NO default-sink
   fallback (explicit decision).
4. `Stop()`: destroy stream/core, close fd; store/load the restore token under XDG data dir.

## Part 4 — Permissions (Linux)

Create `internal/adapter/system/permissions_linux.go` (`//go:build linux`); narrow
`permissions_other.go` to `!darwin && !linux`.

- `microphone` -> `org.freedesktop.portal.Camera`: Status = portal available + mic session usable
  (or honest "granted on first successful mic open"); Request = portal Camera flow.
- `screen-recording` -> `org.freedesktop.portal.ScreenCast`: Status = valid restore token /
  persisted session; Request = run the ScreenCast flow (the picker IS the grant dialog). Replaces
  false `true`.
- `accessibility` -> honest "not supported" (no Linux equivalent), never `true`.
- `OpenSettings`: open a sensible target if one exists; else honest "no settings UI" error. Add a
  linux branch to `settingsURL()` in `permissions.go`.
- Never lie in Status().

## Part 5 — frontend + wiring

- `frontend/src/features/audio/useAudio.ts` guesses the permission by string-matching backend errors
  ("microphone"/"screen-recording") — wrong on Linux. Make the backend surface the permission key
  as structured data in `app:error`/`app:permission` payloads, and have the frontend use it instead
  of string-matching. Keep macOS behavior intact.
- No new UI panel: the portal dialog is the consent UX.
- Do NOT add any new event that duplicates the future `transcription:done` behavior; the STT port
  signature is left untouched by this session.

## ADR / docs

- Amend `docs/adr/005`: Linux audio = second frozen cgo exception (pulse-simple + minimal PipeWire
  portal-node bridge), same freeze rules as whisper.cpp. Update AGENTS.md "single cgo place" wording.
- New ADR `docs/adr/NNN-linux-audio-capture.md`: mic = pulse default source; system = XDG ScreenCast
  audio stream (portal picker); permission mapping; restore-token persistence; no default-sink
  fallback (deliberate).
- ADR-007/008: add a note that macOS keeps TCC/SCK; Linux uses portals. Do NOT rework ADR-007's
  phrase-end section (Step 2 owns that).

## Verification

- Go: `go vet ./...`; `go fmt ./...`; `go test ./...` (needs `scripts/build-whisper.sh` first, as-is
  today). New unit tests for permission mapping and device-shape mapping (mocked). Portal flow is
  manual (below).
- Manual on this machine (PipeWire 1.6.8, Hyprland):
  1. Start mic listening -> confirm PCM reaches STT (`INTERAGENT_STT_DEBUG_RMS=1`, watch
     `/tmp/interagent-stt-rms.log` respond to speech, or see `transcription:done` events).
  2. Start system listening -> portal picker appears -> select audio -> interviewer app audio
     transcribes.
  3. Cancel the portal dialog -> honest error, no crash.
  4. `accessibility` status shows unsupported, not granted.
- Frontend: `cd frontend && pnpm lint && pnpm format:check && pnpm test`.
- `pnpm docs:format:check`.

## Merge note

Merge this branch FIRST. Step 2 (whisper/transcription) will rebase onto your merge. Keep your
branch additive: do not change `internal/adapter/stt/**`, the `STT` port signature, or
`internal/usecase/audiopipeline.go`'s onDone/onPartial wiring.

## Iteration: what actually shipped (2026-09)

Implemented on branch `feat/step1-linux-audio` under test-driven, review-gated
task breakdown. This section records where reality diverged from the plan above.

### Delivery (all Tasks 0-7 accepted after review)

- Microphone: `libpulse-simple` cgo (`capture_pulse_linux.go`), seams in
  `capture_linux.go` (`openPulseReader`/`listPulseSources`/`PulseReachable`),
  honest `!cgo` fallback, ~100 ms float32-mono-48k chunks.
- System sound: XDG Desktop Portal ScreenCast over godbus (`internal/adapter/portal`,
  pure Go) -> PipeWire cgo audio capture (`capture_pipewire_linux.go`), fd ownership
  transferred via `Stream.TakeFD()` (never double-closed).
- Permissions (linux): `Status(accessibility)` is structurally incapable of `true`;
  mic == `PulseReachable()` probe; screen == portal `Granted()`/restore token.
  Per-platform `settingsURL` constants.
- Wiring: `app.go` `NewApp` + `StartListening` emit structured `app:error`
  `{stage:"permission", permission, error}`; `useAudio.ts` consumes it (string-matching
  removed; darwin behavior byte-identical).
- Docs: ADR-013 (linux audio capture; camera-portal mic flow deferred to Step 2),
  ADR-005 amendment (second frozen cgo exception), ADR-007/008 notes, AGENTS.md,
  linux CI job in .github/workflows/test.yml.

### Corrections discovered during execution (facts that override the plan text)

- Camera-portal mic permission was NOT implemented: mic is an honest probe, not a
  consent flow -> deferred to Step 2 (ADR-013 explicit).
- godbus result values arrive as `dbus.Variant`: unwrap with `.Value()` before
  parsing (`parseStreams(results["streams"].Value())`).
- `Start` stream payload is `a(ua{sv})` -> `[]interface{}` of `{uint32, a{sv}}`.
- godbus v5.1.0 lacks `WithMatchPath`/`DetachSignal`/`NameHasOwner`: use
  `WithMatch{Interface,Member,Sender}` + `RemoveSignal` + own `GetNameOwner` probe.
- `CreateSession` on modern portals replies `(o)` request handle; the session handle
  arrives in the Request::Response results, not the `(oo)` reply the plan assumed.
- Threaded-mainloop code must signal on every callback path and `stop` before
  `unlock` (both latent deadlocks in the plan's cgo were fixed at runtime).
- `pa_simple_free` from the caller while the pump is inside `pa_simple_read` is a
  use-after-free: the pump owns the reader lifecycle and `Stop`/`SetDevice` join it
  first (fix in `fix(linux): join pump...`), guarded by a blocking-reader regression test.
- Linux system capture `Devices()` returns `(nil,nil)` until node-info decoding lands
  (honest, dated; no usecase/bind call site depends on it).

### Manual verification still outstanding

- First interactive portal grant on the build machine (picker consent); then persist
  the restore token for headless re-verification (plan Step 6).
- Live pulse reader test needs the daemon's default source to be a working mic.
