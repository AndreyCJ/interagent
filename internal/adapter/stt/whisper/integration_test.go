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
	_ = w.Close()

	select {
	case <-doneCh:
	case <-time.After(30 * time.Second):
		t.Fatal("no transcription:done within 30s")
	}
	if err := <-streamErr; err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	res := <-doneCh
	if res.text == "" {
		t.Fatal("empty transcription for jfk.wav")
	}
	if res.conf <= 0 || res.conf > 1 {
		t.Errorf("confidence = %v, want (0,1]", res.conf)
	}
	if res.lang != "en" {
		t.Errorf("language = %q, want en", res.lang)
	}
}
