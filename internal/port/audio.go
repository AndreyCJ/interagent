package port

// AudioSource identifies which physical source a pipeline captures. It maps
// directly to the conversation role (ADR-007): system → interviewer, mic → user.
type AudioSource string

const (
	AudioSourceSystem AudioSource = "system"
	AudioSourceMic    AudioSource = "mic"
)

// AudioInput captures a single audio source. One instance per source
// (ScreenCaptureKit for system sound, AVCaptureDevice for the microphone).
type AudioInput interface {
	// Start begins capture and delivers raw PCM chunks (little-endian
	// float32, mono, at the device sample rate) to onChunk.
	Start(onChunk func([]byte)) error
	Stop() error
	Devices() ([]AudioDevice, error)
	SetDevice(id string) error
}

// STT transcribes a continuous stream. Chunks arrive via Feed (pumped by the
// pipeline from AudioInput.Start's onChunk); Stream is a blocking method that
// runs until Close.
type STT interface {
	// Feed queues raw PCM bytes (little-endian float32, mono, sampleRate from Stream).
	Feed(chunk []byte) error
	// Stream processes the stream. onPartial fires with cumulative phrase text,
	// onDone with a finalized phrase. Returns nil on Close, an error otherwise.
	Stream(sampleRate int, onPartial func(string), onDone func(text string, confidence float64, language string)) error
	Close() error
}
