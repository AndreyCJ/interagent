package whisper

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStream_RealModel runs only when INTERAGENT_WHISPER_MODEL and
// INTERAGENT_VAD_MODEL are set (CI downloads ggml-tiny.bin + ggml-silero-v6.2.0.bin).
func TestStream_RealModel_Integration(t *testing.T) {
	modelPath := os.Getenv("INTERAGENT_WHISPER_MODEL")
	vadPath := os.Getenv("INTERAGENT_VAD_MODEL")
	if modelPath == "" || vadPath == "" {
		t.Skip("set INTERAGENT_WHISPER_MODEL and INTERAGENT_VAD_MODEL to run")
	}

	wav := filepath.Join("testdata", "jfk.wav")
	data, err := os.ReadFile(wav)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	samples, err := decodeWavFloat32(data)
	if err != nil {
		t.Fatalf("decode wav: %v", err)
	}

	w := New(modelPath, vadPath, "auto")
	doneCh := make(chan struct {
		text string
		conf float64
		lang string
	}, 1)
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- w.Stream(16000, nil, func(text string, confidence float64, language string) {
			select {
			case doneCh <- struct {
				text string
				conf float64
				lang string
			}{text, confidence, language}:
			default:
			}
		})
	}()

	// Feed the wav in 0.1s chunks (1600 samples each).
	chunk := float32ToBytes(samples)
	const chunkSamples = 1600
	for i := 0; i < len(chunk); i += chunkSamples * 4 {
		end := i + chunkSamples*4
		if end > len(chunk) {
			end = len(chunk)
		}
		if err := w.Feed(chunk[i:end]); err != nil {
			t.Fatalf("Feed() error: %v", err)
		}
	}
	// A slow cold model load can leave the stream goroutine still starting up
	// after the whole wav is already queued; it would then burst through the
	// backlog and the gate would never see the quiet run out in real time. Wait
	// for it to catch up so the paced silence below is consumed as it is fed.
	catchUp := time.Now().Add(30 * time.Second)
	for len(w.feed) > 0 {
		if time.Now().After(catchUp) {
			t.Fatal("stream goroutine never consumed the wav feed")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The silence gate only fires when trailing quiet has really elapsed, so
	// feed paced silence after the audio: the phrase finalizes, then nothing
	// more (idle silence produces no Process).
	silence := float32ToBytes(make([]float32, 1600))
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := w.Feed(silence); err != nil {
			t.Fatalf("Feed() silence error: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = w.Close()

	var res struct {
		text string
		conf float64
		lang string
	}
	select {
	case res = <-doneCh:
	case <-time.After(30 * time.Second):
		t.Fatal("no transcription:done within 30s")
	}
	if err := <-streamErr; err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if res.text == "" {
		t.Fatal("empty transcription for jfk.wav")
	}
	if res.conf <= 0 || res.conf > 1 {
		t.Errorf("confidence = %v, want (0,1]", res.conf)
	}
	if res.lang != "en" {
		t.Errorf("language = %q, want en", res.lang)
	}

	select {
	case extra := <-doneCh:
		t.Fatalf("unexpected second transcription:done: %+v", extra)
	case <-time.After(500 * time.Millisecond):
	}
}
