package main

import "interagent/internal/port"

type AgentBind struct {
	usecase interface {
		List() ([]port.AgentConfig, error)
		GetActive() (port.AgentConfig, error)
		SetActive(id string) error
		Save(cfg port.AgentConfig) (port.AgentConfig, error)
		Delete(id string) error
	}
}

func NewAgentBind(u interface {
	List() ([]port.AgentConfig, error)
	GetActive() (port.AgentConfig, error)
	SetActive(id string) error
	Save(cfg port.AgentConfig) (port.AgentConfig, error)
	Delete(id string) error
}) *AgentBind {
	return &AgentBind{usecase: u}
}

func (b *AgentBind) GetAgents() ([]port.AgentConfig, error) {
	return b.usecase.List()
}

func (b *AgentBind) GetActiveAgent() (port.AgentConfig, error) {
	return b.usecase.GetActive()
}

func (b *AgentBind) SetActiveAgent(id string) error {
	return b.usecase.SetActive(id)
}

func (b *AgentBind) SaveAgent(cfg port.AgentConfig) (port.AgentConfig, error) {
	return b.usecase.Save(cfg)
}

func (b *AgentBind) DeleteAgent(id string) error {
	return b.usecase.Delete(id)
}
