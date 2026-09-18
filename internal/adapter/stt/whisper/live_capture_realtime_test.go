package whisper

import (
	"os"
	"testing"
	"time"
)

// TestLiveCaptureRealtimePacing replays the saved raw float32-mono-48k capture
// (/tmp/live-system.pcm) through Stream(48000) at REAL-TIME chunk pacing
// (4800-sample chunks, 100ms steady) — the exact delivery rhythm of the
// capture pump — to determine whether the degraded live transcription
// (*Mumbling*/music markers) is caused by the gate's wall-clock timing on
// continuously-arriving audio rather than by the audio content itself.
//
// Contrast with TestLiveCaptureOfflineProbe, which feeds the same file at ~1.25x
// realtime and transcribes perfectly: if this run degrades to noise markers,
// the root cause is the silence-gate cadence interacting with real-time chunk
// delivery.
func TestLiveCaptureRealtimePacing(t *testing.T) {
	if os.Getenv("INTERAGENT_LIVE_E2E") == "" {
		t.Skip("set INTERAGENT_LIVE_E2E=1 to replay /tmp/live-system.pcm")
	}
	modelPath := os.Getenv("INTERAGENT_WHISPER_MODEL")
	vadPath := os.Getenv("INTERAGENT_VAD_MODEL")
	if modelPath == "" || vadPath == "" {
		t.Skip("set INTERAGENT_WHISPER_MODEL and INTERAGENT_VAD_MODEL to run")
	}

	raw, err := os.ReadFile("/tmp/live-system.pcm")
	if err != nil {
		t.Fatalf("read /tmp/live-system.pcm: %v", err)
	}
	samples := bytesToFloat32(raw) // 48k mono

	var committed, done []string
	w := New(modelPath, vadPath, "auto")
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- w.Stream(48000, nil,
			func(text string) { committed = append(committed, text) },
			func(text string, _ float64, _ string) { done = append(done, text) }, nil)
	}()

	// 4800-sample (100ms) chunks at 48k, delivered on a 100ms cadence.
	const chunkSamples = 4800
	frame := float32ToBytes(samples)
	for i := 0; i < len(frame); i += chunkSamples * 4 {
		end := i + chunkSamples*4
		if end > len(frame) {
			end = len(frame)
		}
		_ = w.Feed(frame[i:end])
		time.Sleep(100 * time.Millisecond)
	}
	// Trailing silence so the final phrase can finalize.
	silence := float32ToBytes(make([]float32, chunkSamples))
	for i := 0; i < 5; i++ {
		_ = w.Feed(silence)
		time.Sleep(100 * time.Millisecond)
	}
	_ = w.Close()
	select {
	case err := <-streamErr:
		if err != nil {
			t.Fatalf("Stream() error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Stream never returned")
	}

	t.Logf("committed: %v", committed)
	t.Logf("done: %v", done)
	t.Logf("total: %d events", len(committed)+len(done))
}
