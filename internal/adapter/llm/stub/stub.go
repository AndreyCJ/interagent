package stub

import (
	"errors"

	"interagent/internal/port"
)

type Stub struct{}

func New() *Stub { return &Stub{} }

func (s *Stub) Complete(prompt string, history []port.Message) (string, error) {
	if prompt == "" {
		return "", errors.New("empty prompt")
	}
	return "stub answer for: " + prompt, nil
}

func (s *Stub) Cancel() error { return nil }
