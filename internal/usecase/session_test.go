package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockSessionStore struct {
	sessions map[string]port.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]port.Session)}
}

func (m *mockSessionStore) CreateSession(s port.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) GetSession(id string) (port.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return port.Session{}, nil
	}
	return s, nil
}

func (m *mockSessionStore) UpdateSession(s port.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) DeleteSession(id string) error {
	delete(m.sessions, id)
	return nil
}

func TestSession_Create(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	session, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if session.ID == "" {
		t.Error("Create() returned session with empty ID")
	}
	if session.ChatHistory == nil {
		t.Error("Create() returned session with nil ChatHistory")
	}
	if session.StartedAt == 0 {
		t.Error("Create() returned session with zero StartedAt")
	}
}

func TestSession_Get_ReturnsCreatedSession(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	created, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Get() returned session with different ID: got %q, want %q", got.ID, created.ID)
	}
}

func TestSession_Clear_RemovesHistory(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	created, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	err = s.Clear(created.ID)
	if err != nil {
		t.Fatalf("Clear() returned error: %v", err)
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() after Clear returned error: %v", err)
	}
	if len(got.ChatHistory) != 0 {
		t.Errorf("after Clear(), ChatHistory should be empty, got %d items", len(got.ChatHistory))
	}
}
