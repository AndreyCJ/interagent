package port

type SessionStorage interface {
	CreateSession(s Session) error
	GetSession(id string) (Session, error)
	UpdateSession(s Session) error
	DeleteSession(id string) error
}

type AgentStorage interface {
	GetAgents() ([]AgentConfig, error)
	SaveAgent(cfg AgentConfig) error
	DeleteAgent(id string) error
}

type SettingsStorage interface {
	GetSettings() (AppSettings, error)
	SaveSettings(s AppSettings) error
}
