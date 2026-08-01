package usecase

import (
	"time"

	"github.com/google/uuid"

	"interagent/internal/port"
)

type Session struct {
	store port.SessionStorage
}

func NewSession(store port.SessionStorage) *Session {
	return &Session{store: store}
}

func (s *Session) Create() (port.Session, error) {
	session := port.Session{
		ID:          uuid.NewString(),
		ChatHistory: []port.Message{},
		StartedAt:   time.Now().Unix(),
	}
	if err := s.store.CreateSession(session); err != nil {
		return port.Session{}, err
	}
	return session, nil
}

func (s *Session) Get(id string) (port.Session, error) {
	return s.store.GetSession(id)
}

func (s *Session) Clear(id string) error {
	session, err := s.store.GetSession(id)
	if err != nil {
		return err
	}
	session.ChatHistory = []port.Message{}
	return s.store.UpdateSession(session)
}
