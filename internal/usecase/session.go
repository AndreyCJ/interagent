package usecase

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"interagent/internal/port"
)

type Session struct {
	store     port.SessionStorage
	currentID string
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
	s.currentID = session.ID
	return session, nil
}

// EnsureSession creates a session if none is current. Called once at app
// startup so GetCurrent/AppendMessage never fail with "no session"
// (Session.currentID is in-memory and empty after launch).
func (s *Session) EnsureSession() error {
	if s.currentID != "" {
		return nil
	}
	_, err := s.Create()
	return err
}

func (s *Session) Get(id string) (port.Session, error) {
	return s.store.GetSession(id)
}

func (s *Session) GetCurrent() (port.Session, error) {
	if s.currentID == "" {
		return port.Session{}, errors.New("no session")
	}
	return s.store.GetSession(s.currentID)
}

func (s *Session) AppendMessage(role, text string) error {
	if role != "user" && role != "assistant" && role != "interviewer" {
		return errors.New("invalid role")
	}
	if text == "" {
		return errors.New("empty text")
	}
	if s.currentID == "" {
		return errors.New("no session")
	}
	cur, err := s.store.GetSession(s.currentID)
	if err != nil {
		return err
	}
	cur.ChatHistory = append(cur.ChatHistory, port.Message{
		Role:      role,
		Text:      text,
		Timestamp: time.Now().UnixMilli(),
	})
	return s.store.UpdateSession(cur)
}

func (s *Session) Clear(id string) error {
	cur, err := s.store.GetSession(id)
	if err != nil {
		return err
	}
	cur.ChatHistory = []port.Message{}
	return s.store.UpdateSession(cur)
}
