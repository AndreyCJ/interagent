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
	return errors.New("not implemented")
}

func (l *LLM) Cancel() error {
	return errors.New("not implemented")
}
