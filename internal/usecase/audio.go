package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Audio struct {
	input port.AudioInput
	stt   port.STT
}

func NewAudio(input port.AudioInput, stt port.STT) *Audio {
	return &Audio{input: input, stt: stt}
}

func (a *Audio) StartListening() error {
	return errors.New("not implemented")
}

func (a *Audio) StopListening() error {
	return errors.New("not implemented")
}

func (a *Audio) IsListening() bool {
	return false
}

func (a *Audio) GetDevices() ([]port.AudioDevice, error) {
	return nil, errors.New("not implemented")
}

func (a *Audio) SetDevice(id string) error {
	return errors.New("not implemented")
}
