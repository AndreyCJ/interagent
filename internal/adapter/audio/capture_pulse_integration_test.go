//go:build linux && cgo

package audio

import (
	"os"
	"testing"
	"time"
)

// TestLivePulseReader opens the default mic for ~1.2 s and asserts we receive
// full float32-mono 48 kHz chunks. Gated; run manually:
// INTERAGENT_PULSE_INTEGRATION=1 go test ./internal/adapter/audio/ -run TestLivePulseReader -v
func TestLivePulseReader(t *testing.T) {
	if os.Getenv("INTERAGENT_PULSE_INTEGRATION") == "" {
		t.Skip("set INTERAGENT_PULSE_INTEGRATION=1 to exercise the live pulse daemon")
	}
	r, err := openPulseReaderCGO("", CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
	if err != nil {
		t.Fatalf("open default mic: %v", err)
	}
	defer r.Close()
	buf := make([]byte, pulseChunkBytes(CaptureSampleRate))
	deadline := time.Now().Add(1200 * time.Millisecond)
	chunks := 0
	for time.Now().Before(deadline) {
		n, err := r.Read(buf)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if n != len(buf) {
			t.Fatalf("read %d bytes, want %d (~100ms float32 mono @48k)", n, len(buf))
		}
		chunks++
		time.Sleep(50 * time.Millisecond)
	}
	if chunks == 0 {
		t.Fatal("no PCM read from default mic")
	}
}

// TestLivePulseSources enumerates the daemon's sources through the cgo
// transporter and asserts the default marked exactly once. It pins the
// threaded-mainloop signalling (enumeration deadlocks without it). Gated:
// INTERAGENT_PULSE_INTEGRATION=1 go test ./internal/adapter/audio/ -run TestLivePulseSources -v
func TestLivePulseSources(t *testing.T) {
	if os.Getenv("INTERAGENT_PULSE_INTEGRATION") == "" {
		t.Skip("set INTERAGENT_PULSE_INTEGRATION=1 to exercise the live pulse daemon")
	}
	srcs, err := listPulseSourcesCGO()
	if err != nil {
		t.Fatalf("listPulseSourcesCGO: %v", err)
	}
	if len(srcs) == 0 {
		t.Fatal("no sources returned by the daemon")
	}
	defaults := 0
	for _, s := range srcs {
		if s.Name == "" || s.Description == "" {
			t.Errorf("source with empty name/description: %+v", s)
		}
		if s.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("got %d default sources, want exactly 1 (%+v)", defaults, srcs)
	}
}
