package port

type LLMInput struct {
	Text     string // prompt text (required)
	Image    []byte // optional: PNG/JPEG of a screenshot (stage 4)
	Format   string // "png" | "jpeg" (if Image != nil)
	Language string // whisper-detected language code, e.g. "en"/"ru"; may be empty
}

type LLM interface {
	Complete(input LLMInput, history []Message, onToken func(string)) (string, error)
	Cancel() error
}
