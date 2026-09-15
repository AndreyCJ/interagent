//go:build linux && cgo

package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"interagent/internal/adapter/audio"
	"interagent/internal/adapter/stt/whisper"
	"interagent/internal/port"
)

// TestLiveSystemTranscription reproduces the app's exact system-sound path
// headlessly: real pulse monitor capture -> real AudioPipeline -> real
// large-v3 + Silero VAD, with a stub LLM and stub history (transcription is
// LLM-independent; only onDone would call the stub, harmlessly). Raw float32
// mono 48k PCM is teed to /tmp/live-system.pcm for offline forensics.
//
// Manual: source scripts/whisper-env.sh && \
//
//	INTERAGENT_LIVE_E2E=1 go test ./internal/usecase/ -run TestLiveSystemTranscription -v
func TestLiveSystemTranscription(t *testing.T) {
	if os.Getenv("INTERAGENT_LIVE_E2E") == "" {
		t.Skip("set INTERAGENT_LIVE_E2E=1 to run against live system audio")
	}
	modelPath := filepath.Join(modelsDirForTest(t), "ggml-large-v3.bin")
	vadPath := filepath.Join(modelsDirForTest(t), "ggml-silero-v6.2.0.bin")
	for name, p := range map[string]string{"model": modelPath, "vad": vadPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s model missing at %s (adopt/download it first): %v", name, p, err)
		}
	}
	tee, err := os.Create("/tmp/live-system.pcm")
	if err != nil {
		t.Fatalf("create /tmp/live-system.pcm: %v", err)
	}
	defer func() { _ = tee.Close() }()
	_ = os.Setenv("INTERAGENT_STT_DEBUG_RMS", "1")

	events := newEventRecorder()
	stt := whisper.New(modelPath, vadPath, "en")
	if err := stt.Preload(); err != nil {
		t.Fatalf("preload stt model: %v", err)
	}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &teeCapture{inner: audio.NewSystemCapture(), tee: tee}, stt, &noopLLM{}, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop() }()

	fmt.Println("LISTENING: play the clip now (up to 35s)")
	deadline := time.Now().Add(35 * time.Second)
	lastLog := time.Time{}
	seenCommitted, seenDone := 0, 0
	for time.Now().Before(deadline) {
		if n := events.count("transcription:committed"); n > seenCommitted {
			for ; seenCommitted < n; seenCommitted++ {
				fmt.Printf("  [%s] COMMITTED %v\n", time.Now().Format("15:04:05"), events.payload("transcription:committed", seenCommitted))
			}
		}
		if n := events.count("transcription:done"); n > seenDone {
			for ; seenDone < n; seenDone++ {
				fmt.Printf("  [%s] DONE %v\n", time.Now().Format("15:04:05"), events.payload("transcription:done", seenDone))
			}
		}
		if n := events.count("app:error"); n > 0 {
			t.Fatalf("app:error surfaced: %v", events.payload("app:error", events.count("app:error")-1))
		}
		if time.Since(lastLog) >= 5*time.Second {
			lastLog = time.Now()
			fmt.Printf("  [%s] committed=%d done=%d\n", time.Now().Format("15:04:05"), seenCommitted, seenDone)
		}
		time.Sleep(100 * time.Millisecond)
	}

	committed := events.count("transcription:committed")
	done := events.count("transcription:done")
	if committed+done == 0 {
		t.Fatalf("no transcription:committed/done within 35s of live system audio — raw capture preserved at /tmp/live-system.pcm, gate log at /tmp/interagent-stt-rms.log")
	}
	var texts []string
	for i := 0; i < committed; i++ {
		texts = append(texts, fmt.Sprintf("committed[%d]=%v", i, events.payload("transcription:committed", i)))
	}
	for i := 0; i < done; i++ {
		texts = append(texts, fmt.Sprintf("done[%d]=%v", i, events.payload("transcription:done", i)))
	}
	fmt.Printf("TRANSCRIBED %d committed + %d done:\n  %s\n", committed, done, joinStrings(texts, "\n  "))
}

func modelsDirForTest(t *testing.T) string {
	t.Helper()
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir: %v", err)
	}
	return filepath.Join(base, "interagent", "models")
}

func joinStrings(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

type noopLLM struct{}

func (n *noopLLM) Generate(port.LLMInput, string) (string, error) { return "", nil }
func (n *noopLLM) Cancel() error                                  { return nil }

// teeCapture forwards every capture chunk both to the pipeline and to a raw
// PCM tee (float32 mono 48k), keeping Stop semantics identical. Every 25
// chunks (~2.5s) it prints a delivery-cadence sample so we can see whether
// pulse delivers smoothly (100ms) or in bursts (which would starve/saturate
// the feed).
type teeCapture struct {
	inner       *audio.SystemCapture
	tee         *os.File
	chunkCount  int
	lastTimings []time.Time
}

func (t *teeCapture) Start(onChunk func([]byte)) error {
	return t.inner.Start(func(chunk []byte) {
		now := time.Now()
		t.chunkCount++
		t.lastTimings = append(t.lastTimings, now)
		if t.tee != nil {
			_, _ = t.tee.Write(chunk)
		}
		onChunk(chunk)
		if t.chunkCount%25 == 0 {
			gaps := make([]string, 0, 24)
			for i := 1; i < len(t.lastTimings); i++ {
				gaps = append(gaps, fmt.Sprintf("%dms", t.lastTimings[i].Sub(t.lastTimings[i-1]).Milliseconds()))
			}
			fmt.Printf("  [%s] %d chunks in window; gaps: %s\n", now.Format("15:04:05.000"), len(t.lastTimings), joinStrings(gaps, " "))
			t.lastTimings = t.lastTimings[:0]
		}
	})
}

func (t *teeCapture) Stop() error { return t.inner.Stop() }
