package port

// ModelStore manages local inference-model artifacts (whisper ggml + Silero VAD).
type ModelStore interface {
	// Status reports whether the model file is installed and its path.
	Status(model string) (installed bool, path string, err error)
	// Download fetches the model, verifying its checksum. onProgress reports
	// cumulative received/total bytes and is called on a goroutine.
	Download(model string, onProgress func(received, total int64)) error
}
