package main

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type stubSessions struct {
	byID    map[string]port.Session
	current string
}

func newStubSessions() *stubSessions {
	return &stubSessions{byID: map[string]port.Session{}}
}

func (s *stubSessions) Create() (port.Session, error) {
	sess := port.Session{ID: "id-1"}
	s.byID[sess.ID] = sess
	return sess, nil
}

func (s *stubSessions) Get(id string) (port.Session, error) {
	sess, ok := s.byID[id]
	if !ok {
		return port.Session{}, errors.New("session not found")
	}
	return sess, nil
}

func (s *stubSessions) GetCurrent() (port.Session, error) {
	if s.current == "" {
		return port.Session{}, errors.New("no session")
	}
	return s.Get(s.current)
}

func (s *stubSessions) Clear(id string) error {
	sess, err := s.Get(id)
	if err != nil {
		return err
	}
	sess.ChatHistory = []port.Message{}
	s.byID[id] = sess
	return nil
}

func TestSessionBindGetSessionReturnsCurrent(t *testing.T) {
	stub := newStubSessions()
	created, err := stub.Create()
	if err != nil {
		t.Fatal(err)
	}
	created.ChatHistory = []port.Message{{Role: "user", Text: "hi"}}
	stub.byID[created.ID] = created
	stub.current = created.ID

	got, err := NewSessionBind(stub).GetSession()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetSession returned session %q, want current %q", got.ID, created.ID)
	}
	if len(got.ChatHistory) != 1 || got.ChatHistory[0].Text != "hi" {
		t.Fatalf("GetSession returned wrong history: %+v", got.ChatHistory)
	}
}

func TestSessionBindClearSessionClearsCurrent(t *testing.T) {
	stub := newStubSessions()
	created, err := stub.Create()
	if err != nil {
		t.Fatal(err)
	}
	created.ChatHistory = []port.Message{{Role: "user", Text: "hi"}}
	stub.byID[created.ID] = created
	stub.current = created.ID

	if err := NewSessionBind(stub).ClearSession(); err != nil {
		t.Fatal(err)
	}
	cur, err := stub.GetCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.ChatHistory) != 0 {
		t.Fatalf("ClearSession left history: %+v", cur.ChatHistory)
	}
}
