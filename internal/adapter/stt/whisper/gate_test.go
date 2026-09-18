package whisper

import (
	"math"
	"testing"
	"time"
)

// gateBase returns the fixed wall-clock anchor the unit tests drive the gate
// with. The gate derives its dt from `last`, so tests advance `now` in explicit
// steps and never sleep.
func gateBase() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func closeFloat(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

// drive feeds the gate a tail level over n steps of the given dt, returning the
// first trigger seen (or trigNone).
func drive(g *silenceGate, now time.Time, dt time.Duration, samples []float32) (time.Time, processTrigger) {
	for _, tail := range samples {
		now = now.Add(dt)
		if trig := g.evaluate(now, tail); trig != trigNone {
			return now, trig
		}
	}
	return now, trigNone
}

// TestGate_AmbientNoise_LocksFloorAndStaysSilent: a fixed ambient level (mic
// hiss, quiet room tone) must end up ruled as idle with the floor tracking it —
// even though it sits far above the old absolute 0.005 gate. The first samples
// can mis-speak because the floor starts unlearned; the flat-tail cold recovery
// is exactly what walks the gate off that phantom phrase and lets the floor
// catch up.
func TestGate_AmbientNoise_LocksFloorAndStaysSilent(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	_, trig := drive(g, now, 200*time.Millisecond, repeat(0.02, 10))
	if trig != trigNone {
		t.Fatalf("ambient noise produced a trigger: %v", trig)
	}
	if g.speaking {
		t.Error("steady ambient ended as the speaking state")
	}
	if !(g.noiseFloor >= 0.018 && g.noiseFloor <= 0.02) {
		t.Errorf("noiseFloor = %.5f, want tracked to ~0.02 ambient", g.noiseFloor)
	}
}

// TestGate_AmbientRises_WhileIdle_FloorFollows: while idle, the floor must
// chase a rising ambient (music, fan spin-up) — the enter gate stays one factor
// above it, so the new ambient alone is not speech either.
func TestGate_AmbientRises_WhileIdle_FloorFollows(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.02, 8))
	if g.speaking {
		t.Fatal("precondition: settled ambient must be idle")
	}
	_, _ = drive(g, now, 250*time.Millisecond, repeat(0.04, 8))
	if !(g.noiseFloor >= 0.035 && g.noiseFloor <= 0.041) {
		t.Errorf("noiseFloor = %.5f after ambient rise to 0.04, want tracked to ~0.04", g.noiseFloor)
	}
	if g.speaking {
		t.Error("rising ambient ended as speaking")
	}
}

// TestGate_VoiceOverNoise_EntersSpeech: a voice that clears the enter factor
// above ambient must flip the gate into speaking once (and not trigger).
func TestGate_VoiceOverNoise_EntersSpeech(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.02, 8))
	if _, trig := drive(g, now, 200*time.Millisecond, []float32{0.06}); trig != trigNone {
		t.Fatalf("entering speech triggered: %v", trig)
	}
	if !g.speaking {
		t.Fatal("voice above the enter gate did not enter the speaking state")
	}
}

// TestGate_NoiseFloor_PauseStillFinalizes is the regression for the reported
// bug: speech over ambient noise (0.02) followed by a pause BACK TO that same
// ambient (not absolute silence) must still finalize. The old absolute 0.005
// gate never fired here — the tail never dropped below the threshold.
func TestGate_NoiseFloor_PauseStillFinalizes(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.02, 8))
	now, _ = drive(g, now, 200*time.Millisecond, []float32{0.06, 0.07, 0.06, 0.07, 0.06})
	if !g.speaking {
		t.Fatal("precondition: voice over noise must be speaking")
	}
	// Pause to the SAME ambient 0.02: the pause gate (1.3×floor ≈ 0.026) sits
	// above it, so the pause must register and finalize after 400ms.
	now, trig := drive(g, now, 200*time.Millisecond, repeat(0.02, 5))
	if trig != trigFinalize {
		t.Fatalf("pause to ambient over noise never finalized (got %v)", trig)
	}
}

// TestGate_ShortPause_ThenSpeechResetsQuiet: a brief pause below the pause gate
// must NOT finalize on its own, and returned speech must cancel the quiet timer
// so a later, longer pause is measured fresh.
func TestGate_ShortPause_ThenSpeechResetsQuiet(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.02, 8))
	now, _ = drive(g, now, 200*time.Millisecond, []float32{0.06, 0.07, 0.06, 0.07, 0.06})

	// 200ms of pause → no finalize, quietSince armed.
	now, trig := drive(g, now, 200*time.Millisecond, []float32{0.02})
	if trig != trigNone {
		t.Fatalf("200ms pause finalized: %v", trig)
	}
	if g.quietSince.IsZero() {
		t.Fatal("pause did not arm the quiet timer")
	}
	// Speech resumes → quiet timer wiped.
	now, trig = drive(g, now, 200*time.Millisecond, []float32{0.06})
	if trig != trigNone {
		t.Fatalf("resumed speech triggered: %v", trig)
	}
	if !g.quietSince.IsZero() {
		t.Error("resumed speech did not reset the quiet timer")
	}
	// A full 400ms pause now finalizes, measured from ITS start.
	_, trig = drive(g, now, 200*time.Millisecond, repeat(0.02, 3))
	if trig != trigFinalize {
		t.Fatalf("400ms pause after reset did not finalize (got %v)", trig)
	}
}

// TestGate_FlatColdStart_RecoversToIdle: when the very first audio IS the
// ambient (no floor learned yet), the gate cannot tell a constant plateau from
// speech. The flat-tail recovery must drop back to idle WITHOUT finalizing, so
// the floor catches up and the ambient is never classified as a phrase (or,
// worse, a permanent "speaking" that hides the reported no-finalize bug).
func TestGate_FlatColdStart_RecoversToIdle(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	_, trig := drive(g, now, 200*time.Millisecond, repeat(0.05, 12))
	if trig != trigNone {
		t.Fatalf("cold-start ambient triggered: %v", trig)
	}
	if g.speaking {
		t.Error("cold-start ambient ended as the permanent speaking state")
	}
	if !(g.noiseFloor >= 0.048 && g.noiseFloor <= 0.05) {
		t.Errorf("noiseFloor = %.5f, want tracked to the 0.05 ambient", g.noiseFloor)
	}
}

// TestGate_ContinuousSpeech_NoFlatRecovery: speech that pulses (as real speech
// always does) must never be mistaken for a flat plateau — the speaking state
// must hold for the whole phrase and the floor must stay frozen at the
// pre-speech ambient, not chase the words.
func TestGate_ContinuousSpeech_NoFlatRecovery(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.02, 8))
	if !(g.noiseFloor >= 0.018 && g.noiseFloor <= 0.021) {
		t.Fatalf("precondition: floor not settled to ~0.02 (got %.5f)", g.noiseFloor)
	}
	// Alternating word levels — RMS pulses 0.06..0.07, well above varFloor.
	_, trig := drive(g, now, 200*time.Millisecond, []float32{0.06, 0.07, 0.06, 0.07, 0.06, 0.07})
	if trig != trigNone && trig != trigFinalize {
		t.Fatalf("continuous speech mis-triggered: %v", trig)
	}
	if !g.speaking {
		t.Fatal("pulsing speech was dropped mid-phrase (flat-recovery misfire)")
	}
	if !(g.noiseFloor >= 0.019 && g.noiseFloor <= 0.021) {
		t.Errorf("noiseFloor = %.5f while speaking, want frozen at ~0.02 ambient (speech must not become floor)", g.noiseFloor)
	}
}

// TestGate_HysteresisBand_FrozenNoFinalize: while speaking, a tail level inside
// [pauseGate, speechGate) is ambiguous — not a pause yet, not definitely
// speech. The quiet timer must be FROZEN (neither armed nor cancelled), so a
// sustained mid-level hold never finalizes a phrase by itself.
func TestGate_HysteresisBand_FrozenNoFinalize(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	now, _ = drive(g, now, 200*time.Millisecond, repeat(0.03, 8))
	now, _ = drive(g, now, 200*time.Millisecond, []float32{0.1, 0.11, 0.1, 0.11})
	if !g.speaking {
		t.Fatal("precondition: loud voice must be speaking")
	}
	// Alternating 0.038/0.044: both sit inside the band (pauseGate ≈ 0.037,
	// speechGate ≈ 0.045 for the 0.028 ambient floor) with a 0.006 RMS spread,
	// so no cold recovery — and a full second of mid-level tail must not arm
	// the quiet timer that would finalize the phrase.
	now, trig := drive(g, now, 200*time.Millisecond, []float32{0.038, 0.044, 0.038, 0.044, 0.038, 0.044})
	if trig != trigNone {
		t.Fatalf("mid-level hold finalized: %v", trig)
	}
	if !g.quietSince.IsZero() {
		t.Error("mid-level hold armed the quiet timer (the band must freeze it)")
	}
	// Draining it to ambient must now start the quiet measurement fresh.
	_, trig = drive(g, now, 200*time.Millisecond, repeat(0.02, 3))
	if trig != trigFinalize {
		t.Fatalf("drop to ambient after mid-level hold did not finalize (got %v)", trig)
	}
}

// TestGate_PureSilence_NeverSpeaks: an all-zero tail is below absFloor, so the
// gate must never enter speech from pure silence (guards idle-silence behavior).
func TestGate_PureSilence_NeverSpeaks(t *testing.T) {
	g := newSilenceGate(streamDefaults)
	now := gateBase()
	_, trig := drive(g, now, 200*time.Millisecond, repeat(0, 10))
	if trig != trigNone {
		t.Fatalf("pure silence produced a trigger: %v", trig)
	}
	if g.speaking {
		t.Error("pure silence entered the speaking state")
	}
	if g.noiseFloor != streamDefaults.absFloor {
		t.Errorf("noiseFloor = %.5f, want absFloor %.5f", g.noiseFloor, streamDefaults.absFloor)
	}
}

func repeat(v float32, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = v
	}
	return out
}
