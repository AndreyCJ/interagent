package stub

import (
	"testing"

	"interagent/internal/port"
)

func TestStub_Complete_ReturnsResponse(t *testing.T) {
	engine := New()

	answer, err := engine.Complete("What is 2+2?", nil)
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	if answer == "" {
		t.Error("Complete() returned empty answer")
	}
}

func TestStub_Complete_EmptyPrompt_ReturnsError(t *testing.T) {
	engine := New()

	if _, err := engine.Complete("", nil); err == nil {
		t.Error("Complete('') should return error for empty prompt")
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
