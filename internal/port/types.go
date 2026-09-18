package port

type Message struct {
	Role      string
	Text      string
	Timestamp int64
}

type Session struct {
	ID          string
	ChatHistory []Message
	StartedAt   int64
}

type AgentConfig struct {
	ID           string
	Name         string
	Provider     string
	Model        string
	BaseURL      string
	APIKey       string
	SystemPrompt string
	Temperature  float64
}

type STTModelStatus struct {
	Installed bool
	Path      string
}

type AppSettings struct {
	Theme              string
	Language           string
	Shortcuts          []Shortcut
	AutoStartListening bool
	// SttModel is the whisper model size key: "tiny", "base" or "small".
	SttModel string
	// SttLanguage is the whisper language: "auto", "ru" or "en".
	SttLanguage string
}

type Shortcut struct {
	ID      string
	Label   string
	Keys    []string
	Enabled bool
}

type AudioDevice struct {
	ID        string
	Name      string
	IsDefault bool
}
