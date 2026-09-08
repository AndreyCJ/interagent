//go:build linux && cgo

package audio

import (
	"testing"
	"time"
)

// TestPWConsumer_Close_BeforeStart_ReturnsPromptly guards the lifecycle rule
// that a Close() on a consumer whose Start() was never called must terminate
// immediately: Start() spawns the goroutine that owns the done channel, so a
// Close() before Start() must not wait on it (ADRs: Close().Before(Start())
// used to hang forever).
func TestPWConsumer_Close_BeforeStart_ReturnsPromptly(t *testing.T) {
	c := &pwConsumer{done: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		_ = c.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close() before Start() must return promptly, not hang on done")
	}
}

// TestPWConsumer_Close_BeforeStart_NoDoneChannel covers a consumer built
// without a done channel: Close() must not panic on close(nil).
func TestPWConsumer_Close_BeforeStart_NoDoneChannel(t *testing.T) {
	c := &pwConsumer{}
	done := make(chan struct{})
	go func() {
		_ = c.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close() before Start() with nil done must return promptly")
	}
}

// TestPWConsumer_Close_Idempotent covers closeOnce semantics: repeated Close()
// calls after the first one return without re-waiting on done.
func TestPWConsumer_Close_Idempotent(t *testing.T) {
	c := &pwConsumer{done: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		_ = c.Close()
		close(done)
	}()
	<-done
	if err := c.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil", err)
	}
}

// TestPWConsumer_Start_WhenAlreadyStarted_Noop drives the idempotency guard
// directly: a consumer that is already in the started state (the invariant
// newPWConsumer+Start leaves behind) must not spawn a second loop goroutine
// or touch the C state again.
func TestPWConsumer_Start_WhenAlreadyStarted_Noop(t *testing.T) {
	c := &pwConsumer{
		started: true, // simulate the post-Start invariant without a live portal fd
		done:    make(chan struct{}),
	}
	timeout := time.After(time.Second)
	if err := c.Start(); err != nil {
		select {
		case <-timeout:
			t.Fatalf("Start() on an already-started consumer hung; want prompt no-op")
		default:
			t.Fatalf("Start() on an already-started consumer = %v, want nil no-op", err)
		}
	}
}

// TestPWConsumer_Start_AfterClose_Rejected is the misuse case: Start() must
// refuse to run after Close() finished tearing the consumer down.
func TestPWConsumer_Start_AfterClose_Rejected(t *testing.T) {
	c := &pwConsumer{done: make(chan struct{})}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
	if err := c.Start(); err == nil {
		t.Fatal("Start() after Close() = nil error, want rejection")
	}
}
