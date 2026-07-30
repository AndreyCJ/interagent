package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Agent struct {
	store port.Storage
}

func NewAgent(store port.Storage) *Agent {
	return &Agent{store: store}
}

func (a *Agent) List() ([]port.AgentConfig, error) {
	return nil, errors.New("not implemented")
}

func (a *Agent) GetActive() (port.AgentConfig, error) {
	return port.AgentConfig{}, errors.New("not implemented")
}

func (a *Agent) SetActive(id string) error {
	return errors.New("not implemented")
}

func (a *Agent) Save(cfg port.AgentConfig) (port.AgentConfig, error) {
	return port.AgentConfig{}, errors.New("not implemented")
}

func (a *Agent) Delete(id string) error {
	return errors.New("not implemented")
}
