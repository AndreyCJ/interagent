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
	ID            string
	Name          string
	Model         string
	SystemPrompt  string
	Temperature   float64
}

type AppSettings struct {
	Theme              string
	Language           string
	Shortcuts          []Shortcut
	AutoStartListening bool
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
