package main

import "interagent/internal/port"

type AudioBind struct {
	app interface {
		StartListening() error
		StopListening() error
		IsListening() bool
		GetAudioDevices() ([]port.AudioDevice, error)
		SetAudioDevice(id string) error
	}
}

func NewAudioBind(app interface {
	StartListening() error
	StopListening() error
	IsListening() bool
	GetAudioDevices() ([]port.AudioDevice, error)
	SetAudioDevice(id string) error
}) *AudioBind {
	return &AudioBind{app: app}
}

func (b *AudioBind) StartListening() error { return b.app.StartListening() }
func (b *AudioBind) StopListening() error  { return b.app.StopListening() }
func (b *AudioBind) IsListening() bool     { return b.app.IsListening() }
func (b *AudioBind) GetAudioDevices() ([]port.AudioDevice, error) {
	return b.app.GetAudioDevices()
}
func (b *AudioBind) SetAudioDevice(id string) error { return b.app.SetAudioDevice(id) }
