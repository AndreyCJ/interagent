package usecase

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type mockAudioInput struct {
	startErr  error
	stopErr   error
	devices   []port.AudioDevice
	setDevice func(id string) error
}

func (m *mockAudioInput) Start() error { return m.startErr }
func (m *mockAudioInput) Stop() error  { return m.stopErr }
func (m *mockAudioInput) Devices() ([]port.AudioDevice, error) {
	return m.devices, nil
}
func (m *mockAudioInput) SetDevice(id string) error {
	if m.setDevice != nil {
		return m.setDevice(id)
	}
	return nil
}

type mockSTT struct{}

func (m *mockSTT) Transcribe(data []byte) (string, float64, error) {
	return "", 0, nil
}

func TestAudio_StartListening(t *testing.T) {
	input := &mockAudioInput{}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	err := a.StartListening()
	if err != nil {
		t.Fatalf("StartListening() returned error: %v", err)
	}
}

func TestAudio_StopListening(t *testing.T) {
	input := &mockAudioInput{}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	err := a.StopListening()
	if err != nil {
		t.Fatalf("StopListening() returned error: %v", err)
	}
}

func TestAudio_IsListening_ReturnsFalseInitially(t *testing.T) {
	input := &mockAudioInput{}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	if a.IsListening() {
		t.Error("IsListening() should return false before StartListening()")
	}
}

func TestAudio_GetDevices_ReturnsDevices(t *testing.T) {
	expected := []port.AudioDevice{
		{ID: "builtin", Name: "Built-in Microphone", IsDefault: true},
	}
	input := &mockAudioInput{devices: expected}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	devices, err := a.GetDevices()
	if err != nil {
		t.Fatalf("GetDevices() returned error: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("GetDevices() returned empty list")
	}
	if devices[0].ID != "builtin" {
		t.Errorf("expected device ID 'builtin', got %q", devices[0].ID)
	}
}

func TestAudio_SetDevice_ValidID(t *testing.T) {
	input := &mockAudioInput{
		setDevice: func(id string) error {
			if id != "external-mic" {
				return errors.New("unknown device")
			}
			return nil
		},
	}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	err := a.SetDevice("external-mic")
	if err != nil {
		t.Fatalf("SetDevice() returned error: %v", err)
	}
}
