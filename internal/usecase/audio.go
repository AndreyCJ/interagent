package usecase

import (
	"interagent/internal/port"
)

type Audio struct {
	input     port.AudioInput
	stt       port.STT
	listening bool
}

func NewAudio(input port.AudioInput, stt port.STT) *Audio {
	return &Audio{input: input, stt: stt}
}

func (a *Audio) StartListening() error {
	if err := a.input.Start(func([]byte) {}); err != nil {
		return err
	}
	a.listening = true
	return nil
}

func (a *Audio) StopListening() error {
	if err := a.input.Stop(); err != nil {
		return err
	}
	a.listening = false
	return nil
}

func (a *Audio) IsListening() bool {
	return a.listening
}

func (a *Audio) GetDevices() ([]port.AudioDevice, error) {
	return a.input.Devices()
}

func (a *Audio) SetDevice(id string) error {
	return a.input.SetDevice(id)
}
