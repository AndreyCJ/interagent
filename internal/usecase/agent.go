package usecase

import (
	"errors"

	"github.com/google/uuid"

	"interagent/internal/port"
)

type Agent struct {
	store    port.AgentStorage
	activeID string
}

func NewAgent(store port.AgentStorage) *Agent {
	return &Agent{store: store}
}

func (a *Agent) List() ([]port.AgentConfig, error) {
	agents, err := a.store.GetAgents()
	if err != nil {
		return nil, err
	}
	if agents == nil {
		return []port.AgentConfig{}, nil
	}
	return agents, nil
}

func (a *Agent) GetActive() (port.AgentConfig, error) {
	if a.activeID != "" {
		agents, err := a.store.GetAgents()
		if err != nil {
			return port.AgentConfig{}, err
		}
		for _, agent := range agents {
			if agent.ID == a.activeID {
				return agent, nil
			}
		}
	}
	// fallback: first stored agent (default) is active
	agents, err := a.store.GetAgents()
	if err != nil {
		return port.AgentConfig{}, err
	}
	if len(agents) > 0 {
		return agents[0], nil
	}
	return port.AgentConfig{}, nil
}

func (a *Agent) SetActive(id string) error {
	a.activeID = id
	return nil
}

func (a *Agent) Save(cfg port.AgentConfig) (port.AgentConfig, error) {
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	if cfg.Provider != "local" && cfg.Provider != "openai-compatible" {
		return port.AgentConfig{}, errors.New("invalid provider: " + cfg.Provider)
	}
	if cfg.Model == "" {
		return port.AgentConfig{}, errors.New("empty model")
	}
	if err := a.store.SaveAgent(cfg); err != nil {
		return port.AgentConfig{}, err
	}
	return cfg, nil
}

func (a *Agent) Delete(id string) error {
	if err := a.store.DeleteAgent(id); err != nil {
		return err
	}
	if a.activeID == id {
		a.activeID = ""
	}
	return nil
}
