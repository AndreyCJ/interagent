package stub

import (
	"errors"

	"interagent/internal/port"
)

type Stub struct{}

func New() *Stub { return &Stub{} }

func (s *Stub) Complete(input port.LLMInput, history []port.Message, onToken func(string)) (string, error) {
	if input.Text == "" {
		return "", errors.New("empty prompt")
	}
	if onToken != nil {
		onToken(input.Text)
	}
	return "stub answer for: " + input.Text, nil
}

func (s *Stub) Cancel() error { return nil }
