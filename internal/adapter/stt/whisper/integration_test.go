package whisper

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		streamErr <- w.Stream(16000, nil, nil, func(text string, confidence float64, language string) {
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
	if strings.ContainsAny(res.text, "[]") {
		t.Errorf("transcription contains control-token garbage: %q", res.text)
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

// TestStream_RealModel_LongForm_CommitsBeforeDone runs three passes of the
// JFK speech back to back (~33 s of near-continuous speech) to exercise the
// committed-segment path against a real model: words from the front of the
// turn must be emitted via onCommitted while speech is still running — i.e.
// before any trailing-silence onDone — instead of being lost at the 5 s / 10 s
// window cap.
func TestStream_RealModel_LongForm_CommitsBeforeDone(t *testing.T) {
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

	var mu sync.Mutex
	committeds := []string{}
	dones := []string{}
	committedAt, doneAt := time.Time{}, time.Time{}
	record := func(kind string, text string) {
		mu.Lock()
		defer mu.Unlock()
		if kind == "committed" {
			committeds = append(committeds, text)
			if committedAt.IsZero() {
				committedAt = time.Now()
			}
		} else {
			dones = append(dones, text)
			if doneAt.IsZero() {
				doneAt = time.Now()
			}
		}
	}

	w := New(modelPath, vadPath, "auto")
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- w.Stream(16000, nil,
			func(text string) { record("committed", text) },
			func(text string, _ float64, _ string) { record("done", text) })
	}()

	frame := float32ToBytes(samples)
	const silenceSamples = 16000 / 5 // 200 ms gap between passes
	silence := float32ToBytes(make([]float32, silenceSamples))
	const chunkSamples = 1600 // 100 ms
	const feedChunk = chunkSamples * 4
	// Pace the feed (~1.6x realtime) so the real-time silence gate can elapse
	// between Process triggers and inference keeps up; a burst would let the
	// window grow unbounded while no trigger fires.
	for pass := 0; pass < 3; pass++ {
		for i := 0; i < len(frame); i += feedChunk {
			end := i + feedChunk
			if end > len(frame) {
				end = len(frame)
			}
			if err := w.Feed(frame[i:end]); err != nil {
				t.Fatalf("Feed() error: %v", err)
			}
			time.Sleep(250 * time.Millisecond)
		}
		if pass < 2 {
			if err := w.Feed(silence); err != nil {
				t.Fatalf("Feed() silence error: %v", err)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	// Trailing silence: a final phrase-boundary Process must produce onDone.
	silenceChunk := float32ToBytes(make([]float32, 1600))
	finalizeAt := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(finalizeAt) {
		if err := w.Feed(silenceChunk); err != nil {
			t.Fatalf("Feed() finalize silence error: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Committed output must arrive before any phrase-boundary done: the front
	// of a long turn is flushed by agreement/cap while speech is continuous.
	deadline := time.Now().Add(20 * time.Second)
	for {
		mu.Lock()
		gotCommitted := len(committeds) > 0
		gotDone := len(dones) > 0
		mu.Unlock()
		if gotCommitted && (gotDone || time.Now().After(deadline)) {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	mu.Lock()
	committedBeforeDone := !doneAt.IsZero() && !committedAt.IsZero() && committedAt.Before(doneAt)
	mu.Unlock()
	if len(committeds) == 0 {
		t.Fatalf("long-form speech produced no onCommitted output (dones=%v)", dones)
	}
	if len(dones) == 0 {
		t.Fatal("long-form speech produced no onDone output")
	}
	if !committedBeforeDone {
		t.Errorf("first onCommitted did not precede first onDone (committedAt=%v doneAt=%v)", committedAt, doneAt)
	}

	// The earliest words of the turn ("ask not what your country…") must
	// survive somewhere in committed + done — nothing is dropped at the cap.
	all := strings.Join(append(append([]string{}, committeds...), dones...), " ")
	low := strings.ToLower(all)
	for _, word := range []string{"ask", "country", "for", "you"} {
		if !strings.Contains(low, word) {
			t.Errorf("long-form transcription lost %q: %s", word, all)
		}
	}
	_ = w.Close()
	if err := <-streamErr; err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
}
