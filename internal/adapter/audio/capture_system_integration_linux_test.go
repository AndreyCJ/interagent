//go:build linux && cgo

package audio

import (
	"context"
	"os"
	"testing"
	"time"

	"interagent/internal/adapter/portal"
)

// TestLiveSystemCapture exercises the full portal picker → PipeWire path.
// Manual: INTERAGENT_PORTAL_INTEGRATION=1 go test ./internal/adapter/audio/ -run TestLiveSystemCapture -v
func TestLiveSystemCapture(t *testing.T) {
	if os.Getenv("INTERAGENT_PORTAL_INTEGRATION") == "" {
		t.Skip("set INTERAGENT_PORTAL_INTEGRATION=1 to run the portal picker flow")
	}
	p := portal.NewScreenCast()
	if !p.Available(context.Background()) {
		t.Skip("no ScreenCast portal available")
	}
	s := NewSystemCapture(p)
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
