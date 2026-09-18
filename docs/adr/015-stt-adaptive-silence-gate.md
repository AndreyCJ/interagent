# ADR-015: Adaptive silence gate with cold/recovery paths

**Date:** 2026-09-18
**Status:** Proposed
**Related documents:** ADR-007, `internal/adapter/stt/whisper/stream.go`, `internal/adapter/stt/whisper/gate_test.go`

---

## Context

whisper.cpp endpoint detection was replaced (ADR-007 amendment 2026-09-09) with an explicit silence
gate in `stream.go`: Process runs only at phrase boundaries the gate decides. The first gate used a
fixed 0.02 RMS `speech_threshold` + `min_silence_ms`; on real captures (default-sink monitor) it
failed both ways — conference audio timed the room's noise floor so a quiet interviewer was never
gated as speech, while deliberate pauses in a mostly-loud call kept re-entering speech. The gate must
(1) track the ambient noise floor per capture and per room, (2) still end a phrase when a speaker
goes quiet mid-phrase, (3) not invent a phrase out of a flat, speech-shaped noise burst (e.g. a stuck
signal), and (4) calibrate from a debug artifact, not by guessing.

## Options

### A. Fixed-threshold gate [rejected]

Original behaviour: single RMS threshold + silence debounce.

Cons: no adaptation to ambient level; quiet speakers + noisy rooms misgate both directions; the
threshold had to be guessed per machine.

### B. Adaptive floor + hysteresis + cold recovery [chosen]

Three independent gates over the tail RMS, fed from the existing model-time RMS ring:

- **Enter-speech:** `speechGate = max(floor·enterFactor, absFloor)` — the tail must rise above it.
- **Exit-speech (hysteresis):** `pauseGate = max(floor·pauseFactor, absFloor)`. While the tail sits in
  `[pauseGate, speechGate)` the pause clock is frozen; it only starts below `pauseGate` and finalizes
  after `finalize = 400 ms` of quiet. This makes a slowly-recovering floor unable to end a phrase.
- **Cold recovery:** a "cold" phrase — one entered on a **unlearned floor** (the very first samples,
  or a sudden loud start) — has no known ambient level; if its tail spread
  (`range = max−min` of the tail ring) stays under `varFloor = 0.004` for `coldRecover = 400 ms`, the
  phrase is dropped as ambient and `Process` is never called. The ring holds the **pre-decision**
  samples (push after the decision), so the enter sample is not erased.

The floor itself adapts **only while idle**: it drops instantly whenever the tail is below it, and
rises toward the **previous** sample only when no tail exceeds the speech gate (2.5×/s toward idle
samples, clamped to `absFloor = 0.005` on the low side). While speaking the floor is fully frozen —
no down-track, no up-track — so mid-phrase quiet does not collapse the pause gate.

Pros: tunes itself to the room/volume; reliable endpoints even in noisy captures; flat ambient or a
stuck signal cannot become a phrase; `INTERAGENT_STT_DEBUG_RMS=1` emits a 1 Hz calibration log
(`tail_rms, floor, speech_gate, pause_gate, speaking, quiet, since_process, energy, window_s, trig`)
to `/tmp/interagent-stt-rms.log`, so tuning is data-driven, not guessed.
Cons: cold-start phrases need the 400 ms recovery check; the floor lags sudden ambient changes by the
rise rate.

## Decision

The **adaptive floor + hysteresis + cold recovery** gate (B). No threshold constants in calibration —
`absFloor`, `enterFactor` (1.6), `pauseFactor` (1.3), `floorRise` (2.5/s), `coldRecover` (400 ms),
`varFloor` (0.004), `finalize` (400 ms). Gate behaviour is pinned by `gate_test.go` (incl. the
stream-level regression: ambient → speech → pause finalizes `Process` exactly once and `onDone`
carries the single word) and the `INTERAGENT_STT_DEBUG_RMS` log for real-room tuning.

## Trade-offs

- A quiet speaker in a _rising_ ambient will see endpoints delayed while the floor catches up; the
  3 s backstop / 4 s cadence still bounds it.
- The floor is frozen during speech — deliberate long pauses render as finalize (400 ms quiet already
  ends the phrase; that is the intended behaviour).
- Flat speech-shaped noise for the first ~1 s of a phrase is dropped (400 ms recovery) — the same
  mechanism that prevents a stuck signal from phoning in a non-phrase.
