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

func TestSession_EnsureSession_CreatesWhenNone(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)
	if err := s.EnsureSession(); err != nil {
		t.Fatalf("EnsureSession() error: %v", err)
	}
	if s.currentID == "" {
		t.Fatal("EnsureSession() did not set a current session")
	}
	if len(store.sessions) != 1 {
		t.Errorf("sessions = %d, want 1", len(store.sessions))
	}
}

func TestSession_EnsureSession_NoopWhenExists(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)
	first, err := s.Create()
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := s.EnsureSession(); err != nil {
		t.Fatalf("EnsureSession() error: %v", err)
	}
	if s.currentID != first.ID {
		t.Errorf("currentID = %q, want %q (must not create a new one)", s.currentID, first.ID)
	}
	if len(store.sessions) != 1 {
		t.Errorf("sessions = %d, want 1", len(store.sessions))
	}
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

func TestSession_Create_SetsCurrent(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	created, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	current, err := s.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() returned error: %v", err)
	}
	if current.ID != created.ID {
		t.Errorf("GetCurrent() ID = %q, want %q", current.ID, created.ID)
	}
}

func TestSession_GetCurrent_WithoutSession_ReturnsError(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	if _, err := s.GetCurrent(); err == nil {
		t.Error("GetCurrent() without created session should return error")
	}
}

func TestSession_AppendMessage_AppendsAndPersists(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	created, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := s.AppendMessage("user", "hello"); err != nil {
		t.Fatalf("AppendMessage(user) returned error: %v", err)
	}
	if err := s.AppendMessage("assistant", "hi there"); err != nil {
		t.Fatalf("AppendMessage(assistant) returned error: %v", err)
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if len(got.ChatHistory) != 2 {
		t.Fatalf("ChatHistory length = %d, want 2", len(got.ChatHistory))
	}
	if got.ChatHistory[0].Role != "user" || got.ChatHistory[0].Text != "hello" {
		t.Errorf("first message = %+v, want user/hello", got.ChatHistory[0])
	}
	if got.ChatHistory[1].Role != "assistant" || got.ChatHistory[1].Text != "hi there" {
		t.Errorf("second message = %+v, want assistant/hi there", got.ChatHistory[1])
	}
	if got.ChatHistory[0].Timestamp == 0 || got.ChatHistory[1].Timestamp == 0 {
		t.Error("messages should have non-zero Timestamp")
	}
}

func TestSession_AppendMessage_InvalidRole_ReturnsError(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	if _, err := s.Create(); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := s.AppendMessage("robot", "hello"); err == nil {
		t.Error("AppendMessage() with invalid role should return error")
	}
}

func TestSession_AppendMessage_EmptyText_ReturnsError(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	if _, err := s.Create(); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := s.AppendMessage("user", ""); err == nil {
		t.Error("AppendMessage() with empty text should return error")
	}
}

func TestSession_Clear_KeepsCurrent(t *testing.T) {
	store := newMockSessionStore()
	s := NewSession(store)

	created, err := s.Create()
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := s.AppendMessage("user", "question"); err != nil {
		t.Fatalf("AppendMessage() returned error: %v", err)
	}
	if err := s.Clear(created.ID); err != nil {
		t.Fatalf("Clear() returned error: %v", err)
	}

	current, err := s.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() returned error: %v", err)
	}
	if len(current.ChatHistory) != 0 {
		t.Errorf("after Clear(), current ChatHistory should be empty, got %d items", len(current.ChatHistory))
	}
}
