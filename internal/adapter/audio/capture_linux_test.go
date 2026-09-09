//go:build linux

package audio

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMicrophoneCapture_Start_WithoutPulseSeam_ReturnsActionableError(t *testing.T) {
	old := openPulseReader
	openPulseReader = nil
	defer func() { openPulseReader = old }()

	m := NewMicrophoneCapture()
	if err := m.Start(func([]byte) {}); err == nil {
		t.Fatal("Start() = nil, want honest error when pulse seam is unavailable")
	}
}

func TestDevices_WithStubbedPulseSources_ReturnsHonestDevices(t *testing.T) {
	oldList := listPulseSources
	listPulseSources = func() ([]pulseSourceInfo, error) {
		return []pulseSourceInfo{
			{Name: "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor", Description: "Built-in Analog Stereo Monitor", IsDefault: true},
			{Name: "bluez_output.CC_22_3D_99_00_11.1", Description: "AirPods Pro", IsDefault: false},
		}, nil
	}
	defer func() { listPulseSources = oldList }()

	devs, err := NewMicrophoneCapture().Devices()
	if err != nil {
		t.Fatalf("Devices() error: %v", err)
	}
	if len(devs) != 2 {
		t.Fatalf("got %d devices, want 2", len(devs))
	}
	if devs[0].ID != "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor" || !devs[0].IsDefault {
		t.Errorf("devs[0] = %+v, want monitor source as default", devs[0])
	}
	if devs[1].Name != "AirPods Pro" || devs[1].IsDefault {
		t.Errorf("devs[1] = %+v, want AirPods Pro non-default", devs[1])
	}
}

func TestPulseReachable_NoSocket_ReturnsHonestError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // no pulse socket present
	if err := pulseReachable(); err == nil {
		t.Fatal("pulseReachable() = nil, want error when no daemon socket")
	}
}

func TestPulseHintErr_UnreachableMapping(t *testing.T) {
	err := pulseHintErr(errors.New("pulse connection refused"))
	if err == nil || !strings.Contains(err.Error(), "pipewire-pulse") {
		t.Errorf("pulseHintErr(connection refused) = %q, want actionable pipewire-pulse hint", err)
	}
}

// blockingReader simulates the live mic: Read blocks (as C.pa_simple_read does
// for a ~100 ms frame) until release is closed, and records the exact sequence
// of state transitions in a mutex-protected log so tests can prove that the
// reader is freed only after Read returned and only before Stop returned.
type blockingReader struct {
	t       *testing.T
	entered chan struct{} // closed when Read starts
	release chan struct{} // closed to unblock the in-flight Read
	ret     chan struct{} // closed right before Read returns
	closed  chan struct{} // closed on Close
	chunk   []byte        // copied into buf on each Read when non-empty

	mu     sync.Mutex
	events []string
	log    func(string) // set by the test to record stop-return on the same timeline
}

func (f *blockingReader) closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

func (f *blockingReader) Read(buf []byte) (int, error) {
	f.mu.Lock()
	f.events = append(f.events, "read-enter")
	f.mu.Unlock()
	f.closeOnce(f.entered)
	<-f.release
	f.mu.Lock()
	f.events = append(f.events, "read-return")
	f.mu.Unlock()
	f.closeOnce(f.ret)
	if len(f.chunk) > 0 && len(f.chunk) <= len(buf) {
		copy(buf, f.chunk)
		return len(f.chunk), nil
	}
	return len(buf), nil
}

func (f *blockingReader) Close() error {
	select {
	case <-f.closed:
		f.mu.Lock()
		f.events = append(f.events, "close-again")
		f.mu.Unlock()
		return nil
	default:
	}
	f.mu.Lock()
	f.events = append(f.events, "close")
	f.mu.Unlock()
	close(f.closed)
	return nil
}

func (f *blockingReader) eventsSince(ev string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.events {
		if e == ev {
			return append([]string(nil), f.events[i+1:]...)
		}
	}
	f.t.Fatalf("event %q missing from log %v", ev, f.events)
	return nil
}

func newBlockingReader(t *testing.T) *blockingReader {
	return &blockingReader{
		t:       t,
		entered: make(chan struct{}),
		release: make(chan struct{}),
		ret:     make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

// TestMicrophoneCapture_Stop_JoinsPumpBeforeFreeingReader is the regression
// test for the Critical review finding: the pump blocks in pa_simple_read (a
// C call that cannot run concurrently with pa_simple_free), so Stop must join
// the pump goroutine and let the pump close the reader itself — it must never
// free the reader from the caller thread while Read is in flight.
func TestMicrophoneCapture_Stop_JoinsPumpBeforeFreeingReader(t *testing.T) {
	oldOpen, oldList := openPulseReader, listPulseSources
	defer func() { openPulseReader, listPulseSources = oldOpen, oldList }()

	fake := newBlockingReader(t)
	openPulseReader = func(dev string, rate, n int) (pulseReader, error) { return fake, nil }
	listPulseSources = func() ([]pulseSourceInfo, error) { return nil, nil }

	m := NewMicrophoneCapture()
	if err := m.Start(func([]byte) {}); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	<-fake.entered // pump is now blocked inside Read, like a live C.pa_simple_read

	select {
	case <-fake.closed:
		t.Fatal("reader freed while the pump is still blocked in Read")
	default:
	}

	fake.log = func(ev string) {
		fake.mu.Lock()
		fake.events = append(fake.events, ev)
		fake.mu.Unlock()
	}
	stopDone := make(chan struct{})
	go func() { _ = m.Stop(); fake.log("stop-return"); close(stopDone) }()

	// Stop cancelled the ctx and is joining; it must not free the reader while
	// Read is still in flight (the pre-fix code did exactly that — Close from
	// the Stop caller thread).
	select {
	case <-fake.closed:
		t.Fatal("Stop freed the reader before the pump goroutine exited Read")
	default:
	}

	close(fake.release) // the frame completes; the pump must exit and free the reader
	<-fake.ret
	<-stopDone
	<-fake.closed

	// Ordering proof (immune to how quickly Stop's cancel propagates, even if
	// the pump read a couple more frames before noticing): every Close must be
	// recorded right after a completed Read — freeing while blocked in Read is
	// the reported Critical bug — and Stop must return only after that Close.
	events := fake.eventsSince("read-enter")
	lastClose := -1
	for i, ev := range events {
		switch ev {
		case "close":
			if i == 0 || events[i-1] != "read-return" {
				t.Fatalf("reader freed while pump mid-Read: lifecycle %v", events)
			}
			lastClose = i
		case "close-again":
			t.Fatalf("reader freed twice: lifecycle %v", events)
		}
	}
	for i, ev := range events {
		if ev == "stop-return" {
			if lastClose == -1 || i < lastClose {
				t.Fatalf("Stop returned before the pump freed the reader: lifecycle %v", events)
			}
			return
		}
	}
	t.Fatalf("stop-return missing from lifecycle %v", events)
}

// TestMicrophoneCapture_SetDevice_RestartsPumpAfterJoiningOldReader verifies
// the device switch routes through the same cancel+join: the old reader is
// freed by the old pump (never mid-Read by SetDevice), and a fresh pump is
// started on the new reader only after the old one fully exited.
func TestMicrophoneCapture_SetDevice_RestartsPumpAfterJoiningOldReader(t *testing.T) {
	oldOpen, oldList := openPulseReader, listPulseSources
	defer func() { openPulseReader, listPulseSources = oldOpen, oldList }()

	oldFake := newBlockingReader(t)
	newFake := newBlockingReader(t)
	openPulseReader = func(dev string, rate, n int) (pulseReader, error) {
		if dev == "" {
			return oldFake, nil
		}
		return newFake, nil
	}
	listPulseSources = func() ([]pulseSourceInfo, error) { return nil, nil }

	m := NewMicrophoneCapture()
	if err := m.Start(func([]byte) {}); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	<-oldFake.entered // old pump blocked in Read

	devDone := make(chan error, 1)
	go func() { devDone <- m.SetDevice("second") }()
	select {
	case <-oldFake.closed:
		t.Fatal("SetDevice freed the old reader while the old pump was blocked in Read")
	default:
	}

	close(oldFake.release) // old pump exits and frees its own reader
	<-oldFake.ret

	if err := <-devDone; err != nil {
		t.Fatalf("SetDevice() error: %v", err)
	}
	<-oldFake.closed // old reader freed only after the old pump exited Read

	// Fresh pump runs on the new reader; nothing was freed mid-Read on either.
	select {
	case <-newFake.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("pump did not restart on the new reader after SetDevice")
	}

	stopDone := make(chan struct{})
	go func() { _ = m.Stop(); close(stopDone) }()
	close(newFake.release)
	<-stopDone
	<-newFake.closed
}

func TestMicrophoneCapture_WithFakeReader_PushesChunkAndStops(t *testing.T) {
	oldOpen, oldList := openPulseReader, listPulseSources
	defer func() { openPulseReader, listPulseSources = oldOpen, oldList }()

	fake := newBlockingReader(t)
	fake.chunk = []byte{0x00, 0x00, 0x80, 0x3f} // one 1.0f float32 sample
	openPulseReader = func(dev string, rate, n int) (pulseReader, error) {
		if dev != "" {
			t.Errorf("openPulseReader device = %q, want empty (default)", dev)
		}
		return fake, nil
	}
	listPulseSources = func() ([]pulseSourceInfo, error) { return nil, nil }

	got := make(chan []byte, 16)
	m := NewMicrophoneCapture()
	if err := m.Start(func(b []byte) { got <- append([]byte(nil), b...) }); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Once release is closed every Read returns instantly, so the pump floods
	// chunks until Stop's cancel lands. A drainer must consume them while Stop
	// joins the pump — otherwise the bounded got channel wedges the pump in a
	// send and the join hangs.
	close(fake.release)
	if _, ok := <-got; !ok {
		t.Fatal("expected at least one chunk")
	}
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range got {
		}
	}()

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	close(got) // pump has exited (Stop joined it); no more sends
	<-drainDone
	select {
	case <-fake.closed:
	default:
		t.Fatal("reader not closed on Stop")
	}
	m2 := NewMicrophoneCapture()
	if err := m2.Stop(); err != nil {
		t.Fatalf("Stop() on unstarted capture: %v", err)
	}
}

func TestSystemCapture_Start_WithoutPortal_ReturnsHonestError(t *testing.T) {
	s := NewSystemCapture(nil)
	if err := s.Start(func([]byte) {}); err == nil {
		t.Fatal("Start() = nil, want honest error when portal is nil")
	}
}

func TestSystemCapture_Stop_Unstarted_Noop(t *testing.T) {
	s := NewSystemCapture(nil)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() on unstarted capture: %v", err)
	}
}

func TestSystemCapture_Start_NilPortal_NamesScreencastPortal(t *testing.T) {
	s := NewSystemCapture(nil)
	err := s.Start(func([]byte) {})
	if err == nil {
		t.Fatal("Start() = nil, want honest error for a nil portal")
	}
	if !strings.Contains(err.Error(), "xdg ScreenCast portal") {
		t.Errorf("Start() error = %q, want it to name the ScreenCast portal", err)
	}
}
