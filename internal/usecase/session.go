package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Session struct {
	store port.Storage
}

func NewSession(store port.Storage) *Session {
	return &Session{store: store}
}

func (s *Session) Create() (port.Session, error) {
	return port.Session{}, errors.New("not implemented")
}

func (s *Session) Get(id string) (port.Session, error) {
	return port.Session{}, errors.New("not implemented")
}

func (s *Session) Clear(id string) error {
	return errors.New("not implemented")
}
