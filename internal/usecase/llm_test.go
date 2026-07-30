package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockLLM struct {
	response string
	err      error
	cancelled bool
}

func (m *mockLLM) Complete(prompt string, history []port.Message) (string, error) {
	return m.response, m.err
}

func (m *mockLLM) Cancel() error {
	m.cancelled = true
	return nil
}

func TestLLM_SendText_ReturnsAnswer(t *testing.T) {
	engine := &mockLLM{response: "This is a helpful answer"}
	llm := NewLLM(engine)

	err := llm.SendText("What is the time complexity of quicksort?")
	if err != nil {
		t.Fatalf("SendText() returned error: %v", err)
	}
}

func TestLLM_Cancel_CancelsGeneration(t *testing.T) {
	engine := &mockLLM{}
	llm := NewLLM(engine)

	err := llm.Cancel()
	if err != nil {
		t.Fatalf("Cancel() returned error: %v", err)
	}
	if !engine.cancelled {
		t.Error("Cancel() did not cancel the underlying engine")
	}
}

func TestLLM_EmptyText_ReturnsError(t *testing.T) {
	engine := &mockLLM{}
	llm := NewLLM(engine)

	err := llm.SendText("")
	if err == nil {
		t.Error("SendText('') should return error for empty text")
	}
}
