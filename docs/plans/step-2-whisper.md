# Session prompt — Step 2: whisper.cpp build de-dup + continuous-speech transcription fix

Repo: Interagent (Wails v2, Go, Vue 3). Follow AGENTS.md strictly: TDD — write tests first, no
implementation without green tests and review. `scripts/build-whisper.sh` before `go test ./...`,
`pnpm --dir frontend build` for go:embed.

PREREQUISITE: Step 1 (Linux audio) has already been merged. This branch is taken from that merge,
so the following are ALREADY DONE and must NOT be clobbered:

- `internal/adapter/audio/capture_linux.go` + pulse/pipwire cgo files, `capture_other.go` narrowed
  to `!darwin && !linux`.
- `internal/adapter/system/permissions_linux.go`, `permissions_other.go` narrowed, linux
  `settingsURL()` branch in `permissions.go`.
- Frontend `useAudio.ts` uses structured permission keys (no string-matching).
- ADR-005 amended (second cgo exception: Linux audio), new `NNN-linux-audio-capture.md` ADR,
  linux notes in ADR-007/008, AGENTS.md "cgo places" wording updated.
  Do NOT touch any of those linux files; only the whisper/transcription scope below.

## Context (verified)

- STT = local whisper.cpp, cgo exception #1 (ADR-005). Module
  `github.com/ggerganov/whisper.cpp/bindings/go` is wired via `go.mod`:
  `replace ... => ./third_party/whisper-bindings`.
- whisper.cpp is a git submodule at `third_party/whisper.cpp` (pinned f049fff9), CMake-built by
  `scripts/build-whisper.sh` into `third_party/whisper.cpp/dist/lib/`.
- STT adapter: `internal/adapter/stt/whisper/` — `whisper.go` (Feed/Stream/Close, byte<->float32,
  linear resample), `stream.go` (silence-gate state machine), `engine.go` (sttContext/sttModel
  seams), tests `whisper_test.go`, `resample_test.go`, `integration_test.go`.
- Port: `internal/port/audio.go` (`STT`). Pipeline: `internal/usecase/audiopipeline.go`
  (system -> auto-answer "interviewer"; mic -> history "user"; cancel-on-new-input ADR-007;
  currently passes `nil` for onPartial). ADRs: `docs/adr/005`, `docs/adr/007`.

## Goal 1 — eliminate the forked binding

Verified: `third_party/whisper-bindings/` is a full copy of the submodule's `bindings/go`
differing ONLY in cgo directives in `whisper.go` (adds `-I` includes + `-L` lib dir). The
submodule's own binding already carries darwin platform flags but lacks include/lib-dir flags.
Step 1's audio cgo files (pulse/pipwire) use per-file `#cgo pkg-config` — they coexist with the
global env vars below, so no conflict.

Steps:

1. Delete `third_party/whisper-bindings/`.
2. `go.mod`: `replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go`;
   `go mod tidy` (keep Step 1's godbus direct dep and any new deps intact).
3. cgo flags via env: in `scripts/build-whisper.sh` (+ CI `.github/workflows/test.yml`, keeping
   Step 1's linux job intact) export `CGO_CFLAGS=-I<abs>/whisper.cpp/include
-I<abs>/whisper.cpp/ggml/include` and `CGO_LDFLAGS=-L<abs>/whisper.cpp/dist/lib -lwhisper
-lggml -lggml-base -lggml-cpu -lm -lstdc++` (add `-fopenmp` on linux) BEFORE `go build`/`go test`.
   Resolve paths absolute from repo root (`${SRCDIR}` only works in package files). Consider moving
   build output from inside the submodule to `build/whisper/lib` (update `.gitignore`).
4. Repo-wide search for `whisper-bindings`/fork refs; update AGENTS.md/README to say the binding
   comes directly from the submodule (do NOT revert Step 1's wording elsewhere).
5. Verify a CLEAN build still links whisper AND Step 1's pulse cgo (i.e. removing the fork didn't
   break linux audio build): `CGO_ENABLED=1 go build ./...` on linux/macos.

## Goal 2 — fix missed words during continuous interviewer speech

Root cause (verified): binding forces `single_segment` whenever a SegmentCallback is set
(`third_party/whisper.cpp/bindings/go/pkg/whisper/context.go` `Process()`); each `whisper_full`
clears prior segment results; so each `Process` re-transcribes the WHOLE 5 s window into ONE
segment. The window is hard-capped (`windowSeconds=5`); older samples dropped. `finalizePhrase()`
emits `onDone` only when the last segment end has >= 400 ms trailing silence (`finalizeSilence`).
During continuous speech: backstop re-processes but `finalizePhrase` no-ops; `partial` stays
overwritten in memory (pipeline passes nil onPartial) -> front of a long turn is permanently lost,
nothing emits until a pause.

Design (parametrizable, pure-Go testable):

1. Replace "5 s drop-oldest window" with a window trimmed relative to the CONFIRMED COMMIT POINT —
   never drop audio from the front before it's committed.
2. Two-pass agreement flush (LocalAgreement-2 style) on each backstop `Process`:
   - keep previous run's segments (with `seg.Start`/`seg.End` timestamps);
   - run `ctx.Process(window, ...)`, collect new segments;
   - longest common prefix of words between previous and current transcription for the same span;
     stable words become COMMITTED;
   - trim the window to just after committed speech end; emit committed text.
3. Adapter callback split (contract change -> amend ADR-007):
   - `onCommitted(text string)` — finalized, non-droppable text pushed mid-speech (history only;
     no LLM);
   - `onDone(text, confidence, language)` — unchanged phrase-boundary emission at >= finalizeSilence
     -> drives auto-answer + cancel-on-new-input exactly as today (ADR-007 untouched for that part);
   - keep `onPartial` for optional future live-draft UI; pipeline may still pass nil.
4. `internal/usecase/audiopipeline.go`: pass `onCommitted` (append to history for both sources, no
   LLM trigger). `onDone` path unchanged. Note this file is the future merge point with Step 1's
   linux work — do not touch its step-1 owned semantics.
5. Preserve VAD + gate timings; do not regress the 10 s-latency / 99%-CPU fix.

Tests (TDD — write first):

- Unit tests with a fake `sttContext` (engine.go seam): continuous speech yields committed output
  before any silence; front-of-window words never silently dropped; phrase `onDone` still fires
  after trailing silence; no double emission between committed and done.
- `integration_test.go` (gated by `INTERAGENT_WHISPER_MODEL`/`INTERAGENT_VAD_MODEL`) passes on
  long-form audio.
- Update stream/state tests that assumed old windowing.

## ADR / docs

- Amend `docs/adr/007-audio-pipeline.md`: document committed-segment emission (final history vs
  live-draft partials; "no partial events" kept, clarified as no live-draft UI partials). Keep
  Step 1's linux notes in the file.
- Amend `docs/adr/005` build note: binding used directly from the submodule (no fork); cgo flags via
  CGO_CFLAGS/CGO_LDFLAGS. Keep Step 1's second-cgo-exception amendment.
- Update AGENTS.md/README build instructions referencing the old path (preserve Step 1 edits).

## Verification

`pnpm --dir frontend build`; `scripts/build-whisper.sh`; `go test ./...`; `go vet ./...`;
`go fmt ./...`; `pnpm docs:format:check`. Also sanity-check a linux build still compiles with the
step-1 pulse cgo files present (`CGO_ENABLED=1 go build ./...`).

## Merge note

This is the LAST merge, applied on top of Step 1. Conflict surface with Step 1 is limited to:
`go.mod` (replace line only), `scripts/build-whisper.sh` + `.github/workflows/test.yml` (keep Step
1's linux job changes, only adjust the CGO env / lib-output section), `AGENTS.md`, `docs/adr/005`,
`docs/adr/007`. Do not revert or overwrite Step 1's linux audio/permissions code in any conflict
resolution.
