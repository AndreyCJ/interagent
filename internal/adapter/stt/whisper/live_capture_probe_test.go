package whisper

import (
	"os"
	"testing"
	"time"
)

// TestLiveCaptureOfflineProbe replays the raw float32-mono-48k capture saved by
// the usecase live E2E (/tmp/live-system.pcm) through the SAME stream +
// resample code the live pipeline uses, but sourced from the file. It isolates
// capture fidelity from the live capture/stream machinery: if the file decodes
// to intelligible speech, the saved PCM is good and the live path is the
// suspect; if it decodes to whisper ambient/music markers, the capture itself
// is corrupt.
//
//	Manual: source scripts/whisper-env.sh && \
//	  INTERAGENT_WHISPER_MODEL=$HOME/.config/interagent/models/ggml-large-v3.bin \
//	  INTERAGENT_VAD_MODEL=$HOME/.config/interagent/models/ggml-silero-v6.2.0.bin \
//	  INTERAGENT_LIVE_E2E=1 go test ./internal/adapter/stt/whisper/ -run TestLiveCaptureOfflineProbe -v
func TestLiveCaptureOfflineProbe(t *testing.T) {
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
	if len(raw) == 0 || len(raw)%4 != 0 {
		t.Fatalf("bad capture size %d", len(raw))
	}
	samples := bytesToFloat32(raw)        // 48k
	rs := resample(samples, 48000, 16000) // exact live downmix path
	t.Logf("resampled %d 48k -> %d 16k samples (%.2f s)", len(samples), len(rs), float64(len(rs))/16000)

	var committed, done []string
	w := New(modelPath, vadPath, "auto")
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- w.Stream(16000, nil,
			func(text string) { committed = append(committed, text) },
			func(text string, _ float64, _ string) { done = append(done, text) }, nil)
	}()

	frame := float32ToBytes(rs)
	const chunkSamples = 1600
	const feedChunk = chunkSamples * 4
	for i := 0; i < len(frame); i += feedChunk {
		end := i + feedChunk
		if end > len(frame) {
			end = len(frame)
		}
		_ = w.Feed(frame[i:end])
		time.Sleep(80 * time.Millisecond) // ~2.5x pacing so the gate can fire
	}
	silence := float32ToBytes(make([]float32, 1600))
	endAt := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(endAt) {
		_ = w.Feed(silence)
		time.Sleep(40 * time.Millisecond)
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
	if len(committed)+len(done) == 0 {
		t.Fatal("offline replay of the live capture produced NO transcription")
	}
}
