package stub

import (
	"testing"

	"interagent/internal/port"
)

func TestStub_Complete_ReturnsResponse(t *testing.T) {
	engine := New()

	answer, err := engine.Complete(port.LLMInput{Text: "hi"}, nil, nil)
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	if answer != "stub answer for: hi" {
		t.Errorf("answer = %q, want %q", answer, "stub answer for: hi")
	}
}

func TestStub_Complete_EmptyPrompt_ReturnsError(t *testing.T) {
	engine := New()

	if _, err := engine.Complete(port.LLMInput{}, nil, nil); err == nil {
		t.Error("Complete('') should return error for empty prompt")
	}
}

func TestStub_Complete_StreamsToken(t *testing.T) {
	engine := New()
	var tokens []string

	answer, err := engine.Complete(port.LLMInput{Text: "hi"}, nil, func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "hi" {
		t.Errorf("onToken called %d times with %v, want 1 time with [hi]", len(tokens), tokens)
	}
	if answer != "stub answer for: hi" {
		t.Errorf("answer = %q, want %q", answer, "stub answer for: hi")
	}
}

func TestStub_Cancel_IsSafe(t *testing.T) {
	engine := New()

	if err := engine.Cancel(); err != nil {
		t.Fatalf("Cancel() returned error: %v", err)
	}
	if err := engine.Cancel(); err != nil {
		t.Fatalf("second Cancel() returned error: %v", err)
	}
}

func TestStub_ImplementsPortLLM(t *testing.T) {
	var _ port.LLM = New()
}
