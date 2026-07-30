package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Settings struct {
	store port.Storage
}

func NewSettings(store port.Storage) *Settings {
	return &Settings{store: store}
}

func (s *Settings) Get() (port.AppSettings, error) {
	return port.AppSettings{}, errors.New("not implemented")
}

func (s *Settings) Save(cfg port.AppSettings) error {
	return errors.New("not implemented")
}

func (s *Settings) UpdateShortcut(id string, keys []string) error {
	return errors.New("not implemented")
}
