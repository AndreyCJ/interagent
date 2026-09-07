//go:build !darwin && !linux

package audio

import (
	"errors"

	"interagent/internal/adapter/portal"
	"interagent/internal/port"
)

type MicrophoneCapture struct{}

func NewMicrophoneCapture() *MicrophoneCapture { return &MicrophoneCapture{} }

func (m *MicrophoneCapture) Start(onChunk func([]byte)) error {
	return errors.New("audio capture is not supported on this platform")
}
func (m *MicrophoneCapture) Stop() error                          { return nil }
func (m *MicrophoneCapture) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (m *MicrophoneCapture) SetDevice(id string) error            { return nil }

type SystemCapture struct{}

func NewSystemCapture(_ *portal.ScreenCast) *SystemCapture { return &SystemCapture{} }

func (s *SystemCapture) Start(onChunk func([]byte)) error {
	return errors.New("system sound capture is not supported on this platform")
}
func (s *SystemCapture) Stop() error                          { return nil }
func (s *SystemCapture) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (s *SystemCapture) SetDevice(id string) error            { return nil }
