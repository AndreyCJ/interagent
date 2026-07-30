package port

type Storage interface {
	CreateSession(s Session) error
	GetSession(id string) (Session, error)
	UpdateSession(s Session) error
	DeleteSession(id string) error

	GetAgents() ([]AgentConfig, error)
	SaveAgent(cfg AgentConfig) error
	DeleteAgent(id string) error

	GetSettings() (AppSettings, error)
	SaveSettings(s AppSettings) error
}
