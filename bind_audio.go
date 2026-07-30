package main

import "interagent/internal/port"

type AudioBind struct {
	usecase interface {
		StartListening() error
		StopListening() error
		IsListening() bool
		GetDevices() ([]port.AudioDevice, error)
		SetDevice(id string) error
	}
}

func NewAudioBind(u interface {
	StartListening() error
	StopListening() error
	IsListening() bool
	GetDevices() ([]port.AudioDevice, error)
	SetDevice(id string) error
}) *AudioBind {
	return &AudioBind{usecase: u}
}

func (b *AudioBind) StartListening() error {
	return b.usecase.StartListening()
}

func (b *AudioBind) StopListening() error {
	return b.usecase.StopListening()
}

func (b *AudioBind) IsListening() bool {
	return b.usecase.IsListening()
}

func (b *AudioBind) GetAudioDevices() ([]port.AudioDevice, error) {
	return b.usecase.GetDevices()
}

func (b *AudioBind) SetAudioDevice(id string) error {
	return b.usecase.SetDevice(id)
}
