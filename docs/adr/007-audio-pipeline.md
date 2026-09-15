# ADR-007: Audio pipeline — sources, phrase end detection, generation cancellation

**Date:** 2026-08-01
**Status:** Accepted
**Related documents:** 01-tz.md §2.1/§3/§8 (stage 3), 02-nfr.md (NFR-01, NFR-02), 04-events.md, 05-architecture.md §3, ADR-001, ADR-005, ADR-008

---

## Context

Scenario 1 (01-tz §3): the app listens to the interviewer and automatically answers their questions. Roles (01-tz §2.1):

- `interviewer` — system sound (the interview/conference app's audio), converted to text via STT;
- `user` — the candidate's voice from the microphone.

Requirements:

- NFR-01: latency from the end of a phrase to the hint — no more than 5 s, target 2 s.
- Both audio sources are processed by local STT (whisper.cpp, ADR-005) — audio never leaves the device (NFR-02).

Not decided in the spec: how to capture system sound on macOS, how to detect "end of phrase", and what to do with new input during response generation.

## Options

### 1. Audio sources

### A. System sound — ScreenCaptureKit (SCK) [chosen]

`SCStream` with an audio descriptor (`SCContentFilter` for system audio) — the only way to capture app/system audio on macOS 14+. Requires screen recording permission (see ADR-008).

**Pros:**

- The official way to capture system audio on macOS 14+.
- Precise app selection (only the interview audio, no background noise).
- Audio stays on the device.

**Cons:**

- macOS only; a bridge via cgo/ObjC is needed (in Go — via `github.com/…` wrappers or a custom cgo adapter). v1 constraint (macOS-only).
- Requires the "Screen Recording" permission (UX handling — ADR-008).

### B. System sound — virtual driver (BlackHole, etc.)

Install a virtual audio device and read it like a microphone.

**Cons:**

- Requires installing a third-party driver — unacceptable for the user. Rejected.

### C. Microphone — native API (AVCaptureDevice) [chosen]

Standard microphone capture (`AVCaptureAudioDevice`/`AVCaptureSession` via a bridge). Devices — via the `AudioDevice` port (already in `internal/port/audio.go`).

### 2. Phrase end detection (for NFR-01)

### D. whisper.cpp endpoint detection [chosen]

The whisper.cpp Go binding (ADR-005) can detect phrase ends in stream mode (`whisper_full` with endpoint/VAD parameters). The stream is cut into phrases; on phrase completion the pipeline starts.

**Pros:**

- Already in the chosen STT (ADR-005), no extra libraries.
- Final phrase → `transcription:done` event (covered by 04-events). Final-only output since 2026-08-07: no partial events — the stream processes at phrase ends (silence gate).

**Cons:**

- Detection quality depends on the VAD model; false boundaries are possible — compensated by sending accumulated context to the LLM.

### E. Threshold energy (RMS) in the audio adapter

**Cons:**

- Unreliable on noisy backgrounds; risk of false triggers. Not the primary mechanism.

### 3. Policy for new input during LLM generation

### F. Cancel and respond to the new input [chosen]

Owner's decision (2026-08-01). When a new phrase completes during generation: `LLM.Cancel()`, then generate a response to the fresh input.

**Pros:**

- The answer always matches the interviewer's latest question.
- No queue accumulates → no growing latency.

**Cons:**

- Partially lost LLM work (CPU/token cost of the cancelled generation).

### G. Queue (FIFO)

**Cons:**

- Latency grows with fast questions; the answer may be stale by the time it is shown. Rejected.

### H. Silence debounce

**Cons:**

- Increases latency (waiting for silence). May be used as an addition to D, not as the primary mechanism.

## Selection criteria

| Criterion        | A (SCK) | B (driver) | D (endpoint) | E (RMS) | F (cancel) | G (queue) |
| ---------------- | ------- | ---------- | ------------ | ------- | ---------- | --------- |
| NFR-01 (2–5 s)   | ✅      | ⚠️         | ✅           | ⚠️      | ✅         | ⚠️        |
| Privacy (NFR-02) | ✅      | ✅         | ✅           | ✅      | ✅         | ✅        |
| User convenience | ✅      | ❌ (inst.) | ✅           | ✅      | ✅         | ⚠️        |
| macOS v1         | ✅      | ⚠️         | ✅           | ✅      | ✅         | ✅        |

## Decision

- **System sound** (`interviewer`): capture via **ScreenCaptureKit**, source selection — the conference app/system audio. Requires screen recording permission (ADR-008).
- **Microphone** (`user`): native macOS API.
- **Phrase end:** endpoint detection in whisper.cpp (stream). The stream processes only at phrase ends (RMS silence gate + cadence backstop); final text goes via `transcription:done` → pipeline start. No partial events.
- **New input during generation:** **Cancel** the current generation (`LLM.Cancel()` → `llm:cancelled` event) and start generation on the fresh input.

### Audio pipeline (stage 3)

```
AudioInput (SCK / mic) → STT whisper.cpp (endpoint detection)
    → transcription:done
    → SendText(text, role) → LLM (cancel on new input) → llm:response
```

### Roles in history

- `transcription:done` from system sound → `Message.Role = "interviewer"`.
- `transcription:done` from microphone → `Message.Role = "user"`.
- LLM response → `Message.Role = "assistant"`.

## Trade-offs

- ScreenCaptureKit is macOS-only (fine for v1); when porting to Windows — an equivalent (WASAPI loopback) in a separate ADR.
- Capturing system sound requires the "Screen Recording" permission — user denial handling in ADR-008.
- Cancelling generation loses the previous request's work — acceptable for the interview UX.
- Endpoint detection can make mistakes — recognition errors are handled per NFR-06 (the user sees a message and can retry).

---

**Linux note (amended 2026-09-10):** system sound is captured from the pulse default-sink monitor
(`@DEFAULT_SINK@.monitor`) with no consent — a native Linux app is not permission-gated; the earlier XDG
ScreenCast portal picker is gone. The mic uses the pulse default source. Otherwise the behaviour is
unchanged: system sound → auto-answer, mic → history. Details: ADR-013.

**Amendment (2026-09-09, committed-segment streaming):** long continuous speech previously lost the
audio beyond the 5 s window front (single_recurrent guess dropped it) — the answer could miss the
correction at the start of a long question / long user turn.

- **Contract split.** `port.STT.Stream` grows a third callback between `onPartial` and `onDone`:
  `onCommitted(text)`. `onPartial` stays unused (no partials, `nil` from the pipeline);
  `onCommitted` fires mid-speech when whisper is confident a prefix of the window will not change;
  `onDone` fires at a phrase end (silence gate), as before.
- **Pipeline routing** (unchanged semantics, new channel): `onCommitted` appends to session history
  only — `interviewer` for system sound, `user` for mic, **no LLM call, no `transcription:done`
  event**. Only `onDone` triggers the auto-answer/`transcription:done` path. Final history for a
  turn = committed words ∪ done (tail) words — each span once, **no duplication**: committed words
  are trimmed out of the window before the finalize run sees the tail.
- **Window never front-drops.** The rolling window is trimmed only at emission points. `maxWindow`
  (10 s) is a **force-commit** cap, not a silent truncation: on overflow, whisper re-runs the whole
  window and commits whatever it transcribes, then trims to the last spoken word.
- **One word lookahead.** A run's words are compared (longest common prefix) with the previous run's
  words (`SetTokenTimestamps(true)` + `NextSegment`, no SegmentCallback — a callback forces the
  binding's `single_segment` mode). All-but-one stable words are committed; the single lookahead
  word keeps finalize able to close the phrase cleanly. Whisper runs only at gate triggers (silence
  finalize 400 ms, cadence backstop 3 s, catch-all 4 s, cap) — still no per-chunk re-transcription.
- **Spans as history.** Word timestamps (`Token.Start/End`) give each word a position relative to
  the window start; commits trim the window at exactly the last committed word's end, and the next
  run's words are re-based so agreement is always position-correct.

**Amendment (2026-09-11, committed words visible in the live chat):** committed mid-speech words were
written to session history only — during a continuous turn the overlay chat showed nothing until a
phrase ended (silence gate), which read to the user as "Listen is broken".

- **New runtime event `transcription:committed`** `{text, source}` ("system" | "mic"), emitted from
  `onCommitted` in parallel with the session-history append. The overlay chat appends an
  `interviewer` (system) or `user` (mic) message on this event, so a long turn is visible live.
- **Semantics unchanged otherwise:** `onCommitted` still never triggers the LLM and still emits no
  `transcription:done`; only `onDone` finalizes the phrase (auto-answer + `transcription:done`).
  The final history for a turn remains committed ∪ done — each span exactly once.
