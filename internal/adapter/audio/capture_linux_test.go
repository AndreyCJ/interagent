//go:build linux

package audio

import (
	"errors"
	"strings"
	"testing"
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

type fakeReader struct {
	start  chan struct{} // closed => reads are ready; signalled to unblock
	chunk  []byte
	closed chan struct{} // closed on Close()
}

func (f *fakeReader) Read(buf []byte) (int, error) {
	<-f.start
	if f.chunk == nil || len(f.chunk) > len(buf) {
		return len(buf), nil
	}
	copy(buf, f.chunk)
	return len(f.chunk), nil
}

func (f *fakeReader) Close() error { close(f.closed); return nil }

func TestMicrophoneCapture_WithFakeReader_PushesChunkAndStops(t *testing.T) {
	oldOpen, oldList := openPulseReader, listPulseSources
	defer func() { openPulseReader, listPulseSources = oldOpen, oldList }()

	fake := &fakeReader{
		start:  make(chan struct{}),
		chunk:  []byte{0x00, 0x00, 0x80, 0x3f}, // one 1.0f float32 sample
		closed: make(chan struct{}),
	}
	openPulseReader = func(dev string, rate, n int) (pulseReader, error) {
		if dev != "" {
			t.Errorf("openPulseReader device = %q, want empty (default)", dev)
		}
		return fake, nil
	}
	listPulseSources = func() ([]pulseSourceInfo, error) { return nil, nil }

	got := make(chan []byte, 64)
	m := NewMicrophoneCapture()
	if err := m.Start(func(b []byte) { got <- append([]byte(nil), b...) }); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	close(fake.start) // reads become ready
	if _, ok := <-got; !ok {
		t.Fatal("expected at least one chunk")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
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
