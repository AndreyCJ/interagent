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
	// Stream processes the stream.
	//
	// onPartial fires with the live draft of the current window (optional; the
	// pipeline passes nil — there is no live-draft UI, ADR-007).
	//
	// onCommitted fires mid-speech with text that is finalized beyond further
	// revision and was pushed to the session history (final, non-droppable).
	// It never fires again for the same span.
	//
	// onDone fires at a phrase boundary (trailing silence) with the remaining
	// uncommitted tail of the phrase — the span NOT already emitted via
	// onCommitted — plus its confidence and detected language. It drives
	// auto-answer and cancel-on-new-input exactly as today (ADR-007).
	//
	// onStatus reports whether whisper is currently processing the window
	// (busy=true at the start of a Process run, busy=false when it returns) —
	// the frontend surfaces it as a transcription loading indicator. Optional;
	// nil-safe.
	Stream(sampleRate int, onPartial func(string), onCommitted func(string), onDone func(text string, confidence float64, language string), onStatus func(bool)) error
	Close() error
}
