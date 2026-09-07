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
