package port

type AudioInput interface {
	Start() error
	Stop() error
	Devices() ([]AudioDevice, error)
	SetDevice(id string) error
}

type STT interface {
	Transcribe(audioData []byte) (string, float64, error)
}
