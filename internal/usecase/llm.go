package usecase

import (
	"errors"

	"interagent/internal/port"
)

type LLM struct {
	engine port.LLM
}

func NewLLM(engine port.LLM) *LLM {
	return &LLM{engine: engine}
}

func (l *LLM) SendText(text string) error {
	if text == "" {
		return errors.New("empty text")
	}
	_, err := l.engine.Complete(text, nil)
	return err
}

func (l *LLM) Cancel() error {
	return l.engine.Cancel()
}
