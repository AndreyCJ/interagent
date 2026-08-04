package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockAudioInput struct {
	onChunk  func([]byte)
	startErr error
	stopErr  error
	started  bool
	stopped  bool
}

func (m *mockAudioInput) Start(onChunk func([]byte)) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.onChunk = onChunk
	m.started = true
	return nil
}

func (m *mockAudioInput) Stop() error { m.stopped = true; return m.stopErr }

func (m *mockAudioInput) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (m *mockAudioInput) SetDevice(id string) error            { return nil }

type mockSTT struct {
	onPartial  func(string)
	onDone     func(string, float64, string)
	sampleRate int
	feedErr    error
	closed     bool
}

func (m *mockSTT) Feed(chunk []byte) error { return m.feedErr }

func (m *mockSTT) Stream(sampleRate int, onPartial func(string), onDone func(string, float64, string)) error {
	m.sampleRate = sampleRate
	m.onPartial = onPartial
	m.onDone = onDone
	return nil
}

func (m *mockSTT) Close() error { m.closed = true; return nil }

func TestAudio_StartListening_FeedsNoopChunks(t *testing.T) {
	input := &mockAudioInput{}
	stt := &mockSTT{}
	a := NewAudio(input, stt)

	if err := a.StartListening(); err != nil {
		t.Fatalf("StartListening() returned error: %v", err)
	}
	if !input.started {
		t.Error("StartListening() did not call input.Start")
	}
	if input.onChunk == nil {
		t.Error("StartListening() must pass an onChunk func to input.Start")
	}
}
