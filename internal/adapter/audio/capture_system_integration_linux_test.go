//go:build linux && cgo

package audio

import (
	"os"
	"testing"
	"time"
)

// TestLiveSystemCapture exercises the pulse default-sink monitor path against
// a live pipewire-pulse / pulseaudio daemon.
// Manual: INTERAGENT_PULSE_INTEGRATION=1 go test ./internal/adapter/audio/ -run TestLiveSystemCapture -v
func TestLiveSystemCapture(t *testing.T) {
	if os.Getenv("INTERAGENT_PULSE_INTEGRATION") == "" {
		t.Skip("set INTERAGENT_PULSE_INTEGRATION=1 to run against a live pulse daemon")
	}
	s := NewSystemCapture()
	chunks := make(chan []byte, 8)
	if err := s.Start(func(b []byte) {
		select {
		case chunks <- append([]byte(nil), b...):
		default:
		}
	}); err != nil {
		t.Fatalf("start system capture: %v", err)
	}
	defer func() { _ = s.Stop() }()
	deadline := time.After(3 * time.Second)
	select {
	case <-chunks:
		// system audio PCM received
	case <-deadline:
		t.Fatal("no system audio chunks within 3s")
	}
}
