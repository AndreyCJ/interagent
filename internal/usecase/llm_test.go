package usecase

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type mockLLM struct {
	response  string
	err       error
	cancelled bool
}

func (m *mockLLM) Complete(prompt string, history []port.Message) (string, error) {
	return m.response, m.err
}

func (m *mockLLM) Cancel() error {
	m.cancelled = true
	return nil
}

type mockSessionWriter struct {
	roles []string
	texts []string
	err   error
}

func (m *mockSessionWriter) AppendMessage(role, text string) error {
	if m.err != nil {
		return m.err
	}
	m.roles = append(m.roles, role)
	m.texts = append(m.texts, text)
	return nil
}

func llmPayload(payload any) string {
	return payload.(map[string]string)["text"]
}

func errorPayload(payload any) string {
	return payload.(map[string]string)["error"]
}

func TestLLM_Generate_EmptyText_ReturnsError(t *testing.T) {
	llm := NewLLM(&mockLLM{}, newMockEvents(), &mockSessionWriter{})
	if _, err := llm.Generate(""); err == nil {
		t.Error("Generate('') should return error for empty text")
	}
}

func TestLLM_Generate_AppendsUser_EmitsStarted_ReturnsAnswer(t *testing.T) {
	engine := &mockLLM{response: "This is the answer"}
	events := newMockEvents()
	session := &mockSessionWriter{}
	llm := NewLLM(engine, events, session)

	answer, err := llm.Generate("What is 2+2?")
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if answer != "This is the answer" {
		t.Errorf("answer = %q, want %q", answer, "This is the answer")
	}
	if events.count("llm:started") != 1 {
		t.Errorf("expected 1 llm:started event, got %d", events.count("llm:started"))
	}
	if events.count("llm:response") != 1 {
		t.Fatalf("expected 1 llm:response event, got %d", events.count("llm:response"))
	}
	if got := llmPayload(events.payload("llm:response", 0)); got != "This is the answer" {
		t.Errorf("llm:response text = %q, want %q", got, "This is the answer")
	}
	if len(session.roles) != 2 || session.roles[0] != "user" || session.roles[1] != "assistant" {
		t.Fatalf("roles = %v, want [user assistant]", session.roles)
	}
	if session.texts[0] != "What is 2+2?" || session.texts[1] != "This is the answer" {
		t.Errorf("texts = %v, want prompt then answer", session.texts)
	}
}

func TestLLM_Generate_SessionError_ReturnsError_NoStarted(t *testing.T) {
	llm := NewLLM(&mockLLM{response: "answer"}, newMockEvents(), &mockSessionWriter{err: errors.New("storage failed")})

	if _, err := llm.Generate("hello"); err == nil {
		t.Error("Generate() should propagate session append error")
	}
}

func TestLLM_Generate_EngineError_EmitsError_NoAssistant(t *testing.T) {
	engine := &mockLLM{err: errors.New("network timeout")}
	events := newMockEvents()
	session := &mockSessionWriter{}
	llm := NewLLM(engine, events, session)

	if _, err := llm.Generate("question"); err == nil {
		t.Fatal("Generate() should return engine error")
	}
	if events.count("llm:error") != 1 {
		t.Fatalf("expected 1 llm:error event, got %d", events.count("llm:error"))
	}
	if got := errorPayload(events.payload("llm:error", 0)); got != "network timeout" {
		t.Errorf("llm:error payload = %q, want %q", got, "network timeout")
	}
	if events.count("llm:response") != 0 {
		t.Error("llm:response should not be emitted on engine error")
	}
	if len(session.roles) != 1 || session.roles[0] != "user" {
		t.Error("assistant message should not be appended on engine error; only the user message should exist")
	}
}

func TestLLM_Generate_NoResponse_EmitsError(t *testing.T) {
	engine := &mockLLM{response: ""}
	events := newMockEvents()
	llm := NewLLM(engine, events, &mockSessionWriter{})

	if _, err := llm.Generate("question"); err == nil {
		t.Fatal("Generate() should return error for empty answer")
	}
	if events.count("llm:error") != 1 {
		t.Fatalf("expected 1 llm:error event, got %d", events.count("llm:error"))
	}
}

func TestLLM_Cancel_CancelsGeneration(t *testing.T) {
	engine := &mockLLM{}
	llm := NewLLM(engine, newMockEvents(), &mockSessionWriter{})

	if err := llm.Cancel(); err != nil {
		t.Fatalf("Cancel() returned error: %v", err)
	}
	if !engine.cancelled {
		t.Error("Cancel() did not cancel the underlying engine")
	}
}
